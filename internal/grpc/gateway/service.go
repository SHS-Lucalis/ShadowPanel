package gateway

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"time"

	"github.com/gameap/gameap/internal/enrollment"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/grpc/session"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

const (
	defaultHeartbeatInterval = 30 * time.Second
	smallFileThreshold       = 1 * 1024 * 1024
)

type Service struct {
	proto.UnimplementedDaemonGatewayServer

	registry       *session.Registry
	nodeRepo       repositories.NodeRepository
	serverRepo     repositories.ServerRepository
	daemonTaskRepo repositories.DaemonTaskRepository
	gameRepo       repositories.GameRepository
	gameModRepo    repositories.GameModRepository
	logger         *slog.Logger
	apiKeyVerifier APIKeyVerifier
	taskHandler    TaskHandler
	commandHandler CommandHandler
	serverHandler  ServerStatusHandler
	attachHandler  AttachHandler
	enrollmentSvc  *enrollment.Service
}

type APIKeyVerifier interface {
	Verify(apiKey string, nodeID uint64) (bool, error)
}

type TaskHandler interface {
	HandleTaskStatusUpdate(ctx context.Context, nodeID uint64, update *proto.TaskStatusUpdate) error
	HandleTaskOutput(ctx context.Context, nodeID uint64, output *proto.TaskOutput) error
	GetPendingTasks(ctx context.Context, nodeID uint64) ([]*proto.DaemonTask, error)
}

type CommandHandler interface {
	HandleCommandOutput(ctx context.Context, nodeID uint64, output *proto.CommandOutput) error
	HandleCommandResult(ctx context.Context, nodeID uint64, result *proto.CommandResult) error
}

type ServerStatusHandler interface {
	HandleServerStatuses(ctx context.Context, nodeID uint64, statuses *proto.ServerStatusBatch) error
}

type AttachHandler interface {
	HandleAttachStarted(ctx context.Context, nodeID uint64, started *proto.AttachStarted) error
	HandleAttachOutput(ctx context.Context, nodeID uint64, output *proto.AttachOutput) error
	HandleAttachClosed(ctx context.Context, nodeID uint64, closed *proto.AttachClosed) error
}

type Config struct {
	HeartbeatInterval int32
}

func NewService(
	registry *session.Registry,
	nodeRepo repositories.NodeRepository,
	serverRepo repositories.ServerRepository,
	daemonTaskRepo repositories.DaemonTaskRepository,
	gameRepo repositories.GameRepository,
	gameModRepo repositories.GameModRepository,
	apiKeyVerifier APIKeyVerifier,
	taskHandler TaskHandler,
	commandHandler CommandHandler,
	serverHandler ServerStatusHandler,
	attachHandler AttachHandler,
	enrollmentSvc *enrollment.Service,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		registry:       registry,
		nodeRepo:       nodeRepo,
		serverRepo:     serverRepo,
		daemonTaskRepo: daemonTaskRepo,
		gameRepo:       gameRepo,
		gameModRepo:    gameModRepo,
		apiKeyVerifier: apiKeyVerifier,
		taskHandler:    taskHandler,
		commandHandler: commandHandler,
		serverHandler:  serverHandler,
		attachHandler:  attachHandler,
		enrollmentSvc:  enrollmentSvc,
		logger:         logger,
	}
}

func (s *Service) Connect(stream proto.DaemonGateway_ConnectServer) error {
	ctx := stream.Context()

	msg, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive registration message")
	}

	reg := msg.GetRegister()
	if reg == nil {
		return status.Error(codes.InvalidArgument, "first message must be RegisterRequest")
	}

	if err := s.validateAuth(ctx, reg); err != nil {
		return err
	}

	sessionCtx, cancel := context.WithCancel(ctx)
	sess := session.NewSession(
		reg.NodeId,
		stream,
		reg.Version,
		reg.Capabilities,
		cancel,
	)

	if err := s.registry.Register(ctx, sess); err != nil {
		cancel()

		return status.Error(codes.Internal, "failed to register session")
	}

	defer func() {
		_ = s.registry.Unregister(context.Background(), reg.NodeId)
	}()

	ack, err := s.buildRegisterAck(ctx, reg)
	if err != nil {
		s.logger.Error("failed to build register ack",
			"node_id", reg.NodeId,
			"error", err,
		)

		return status.Error(codes.Internal, "failed to build registration response")
	}

	if err := stream.Send(&proto.GatewayMessage{
		RequestId: msg.RequestId,
		Payload: &proto.GatewayMessage_RegisterAck{
			RegisterAck: ack,
		},
	}); err != nil {
		return status.Error(codes.Internal, "failed to send registration ack")
	}

	s.logger.Info("daemon connected",
		"node_id", reg.NodeId,
		"version", reg.Version,
		"capabilities", reg.Capabilities,
	)

	return s.handleMessages(sessionCtx, sess)
}

func (s *Service) validateAuth(ctx context.Context, reg *proto.RegisterRequest) error {
	nodes, err := s.nodeRepo.Find(ctx, &filters.FindNode{IDs: []uint{uint(reg.NodeId)}}, nil, nil)
	if err != nil {
		s.logger.Error("failed to find node", "node_id", reg.NodeId, "error", err)

		return status.Error(codes.Internal, "failed to verify node")
	}

	if len(nodes) == 0 {
		return status.Error(codes.NotFound, "node not found")
	}

	node := nodes[0]

	if !node.Enabled {
		return status.Error(codes.PermissionDenied, "node is disabled")
	}

	if s.apiKeyVerifier != nil {
		valid, err := s.apiKeyVerifier.Verify(reg.ApiKey, reg.NodeId)
		if err != nil {
			s.logger.Error("failed to verify API key", "node_id", reg.NodeId, "error", err)

			return status.Error(codes.Internal, "failed to verify API key")
		}
		if !valid {
			return status.Error(codes.Unauthenticated, "invalid API key")
		}
	} else if reg.ApiKey != node.GdaemonAPIKey {
		return status.Error(codes.Unauthenticated, "invalid API key")
	}

	return nil
}

func (s *Service) buildRegisterAck(ctx context.Context, reg *proto.RegisterRequest) (*proto.RegisterAck, error) {
	servers, err := s.serverRepo.Find(ctx, &filters.FindServer{
		DSIDs: []uint{uint(reg.NodeId)},
	}, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "find servers for node")
	}

	protoServers := make([]*proto.Server, 0, len(servers))
	for _, srv := range servers {
		protoServers = append(protoServers, domainServerToProto(&srv))
	}

	var pendingTasks []*proto.DaemonTask
	if s.taskHandler != nil {
		pendingTasks, err = s.taskHandler.GetPendingTasks(ctx, reg.NodeId)
		if err != nil {
			s.logger.Warn("failed to get pending tasks", "node_id", reg.NodeId, "error", err)
		}
	}

	games, err := s.gameRepo.FindAll(ctx, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "find games")
	}

	protoGames := make([]*proto.Game, 0, len(games))
	for _, g := range games {
		if g.Enabled == 0 {
			continue
		}
		protoGames = append(protoGames, domainGameToProto(&g))
	}

	gameMods, err := s.gameModRepo.FindAll(ctx, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "find game mods")
	}

	protoGameMods := make([]*proto.GameMod, 0, len(gameMods))
	for _, gm := range gameMods {
		protoGameMods = append(protoGameMods, domainGameModToProto(&gm))
	}

	s.logger.Debug("register ack prepared",
		"node_id", reg.NodeId,
		"servers", len(protoServers),
		"tasks", len(pendingTasks),
		"games", len(protoGames),
		"game_mods", len(protoGameMods),
	)

	return &proto.RegisterAck{
		Success:           true,
		Servers:           protoServers,
		PendingTasks:      pendingTasks,
		Games:             protoGames,
		GameMods:          protoGameMods,
		HeartbeatInterval: durationpb.New(defaultHeartbeatInterval),
	}, nil
}

func (s *Service) handleMessages(ctx context.Context, sess *session.Session) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msg, err := sess.Stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			s.logger.Error("failed to receive message",
				"node_id", sess.NodeID,
				"error", err,
			)

			return err
		}

		if err := s.processMessage(ctx, sess, msg); err != nil {
			s.logger.Error("failed to process message",
				"node_id", sess.NodeID,
				"error", err,
			)
		}
	}
}

func (s *Service) processMessage(ctx context.Context, sess *session.Session, msg *proto.DaemonMessage) error {
	switch payload := msg.Payload.(type) {
	case *proto.DaemonMessage_Heartbeat:
		sess.UpdateLastPing()

		return nil

	case *proto.DaemonMessage_TaskStatus:
		if s.taskHandler != nil {
			return s.taskHandler.HandleTaskStatusUpdate(ctx, sess.NodeID, payload.TaskStatus)
		}

	case *proto.DaemonMessage_TaskOutput:
		if s.taskHandler != nil {
			return s.taskHandler.HandleTaskOutput(ctx, sess.NodeID, payload.TaskOutput)
		}

	case *proto.DaemonMessage_CommandOutput:
		if s.commandHandler != nil {
			return s.commandHandler.HandleCommandOutput(ctx, sess.NodeID, payload.CommandOutput)
		}

	case *proto.DaemonMessage_CommandResult:
		if payload.CommandResult.RequestId != "" {
			sess.ResolvePendingRequest(payload.CommandResult.RequestId, msg)
		}
		if s.commandHandler != nil {
			return s.commandHandler.HandleCommandResult(ctx, sess.NodeID, payload.CommandResult)
		}

	case *proto.DaemonMessage_ServerStatuses:
		if s.serverHandler != nil {
			s.logger.Info("received server status batch",
				"node_id", sess.NodeID,
				"count", len(payload.ServerStatuses.GetStatuses()),
			)

			return s.serverHandler.HandleServerStatuses(ctx, sess.NodeID, payload.ServerStatuses)
		}

		s.logger.Warn("received server statuses but handler is nil", "node_id", sess.NodeID)

	case *proto.DaemonMessage_FileReadResponse:
		sess.ResolvePendingRequest(payload.FileReadResponse.RequestId, msg)

		return nil

	case *proto.DaemonMessage_FileWriteResponse:
		sess.ResolvePendingRequest(payload.FileWriteResponse.RequestId, msg)

		return nil

	case *proto.DaemonMessage_FileListResponse:
		sess.ResolvePendingRequest(payload.FileListResponse.RequestId, msg)

		return nil

	case *proto.DaemonMessage_FileOperationResponse:
		sess.ResolvePendingRequest(payload.FileOperationResponse.RequestId, msg)

		return nil

	case *proto.DaemonMessage_StatusResponse:
		sess.ResolvePendingRequest(payload.StatusResponse.RequestId, msg)

		return nil

	case *proto.DaemonMessage_AttachStarted:
		if s.attachHandler != nil {
			return s.attachHandler.HandleAttachStarted(ctx, sess.NodeID, payload.AttachStarted)
		}

	case *proto.DaemonMessage_AttachOutput:
		if s.attachHandler != nil {
			return s.attachHandler.HandleAttachOutput(ctx, sess.NodeID, payload.AttachOutput)
		}

	case *proto.DaemonMessage_AttachClosed:
		if s.attachHandler != nil {
			return s.attachHandler.HandleAttachClosed(ctx, sess.NodeID, payload.AttachClosed)
		}

	default:
		s.logger.Warn("unknown message type received",
			"node_id", sess.NodeID,
			"request_id", msg.RequestId,
		)
	}

	return nil
}

func (s *Service) RequestFileRead(
	ctx context.Context,
	nodeID uint64,
	path string,
	offset int64,
	length int64,
) (*proto.FileReadResponse, error) {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return nil, errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_FileRead{
			FileRead: &proto.FileReadRequest{
				Path:   path,
				Offset: offset,
				Length: length,
			},
		},
	}); err != nil {
		return nil, errors.Wrap(err, "send file read request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("request cancelled")
		}
		fileResp := resp.GetFileReadResponse()
		if fileResp == nil {
			return nil, errors.New("unexpected response type")
		}

		return fileResp, nil
	}
}

func (s *Service) RequestFileWrite(
	ctx context.Context, nodeID uint64, path string, content []byte, mode int32, createDirs bool,
) error {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_FileWrite{
			FileWrite: &proto.FileWriteRequest{
				Path:       path,
				Content:    content,
				Mode:       mode,
				CreateDirs: createDirs,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "send file write request")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return errors.New("request cancelled")
		}
		fileResp := resp.GetFileWriteResponse()
		if fileResp == nil {
			return errors.New("unexpected response type")
		}
		if !fileResp.Success {
			return errors.New(fileResp.Error)
		}

		return nil
	}
}

func (s *Service) RequestFileList(
	ctx context.Context, nodeID uint64, path string, recursive bool, pattern string,
) (*proto.FileListResponse, error) {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return nil, errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_FileList{
			FileList: &proto.FileListRequest{
				Path:      path,
				Recursive: recursive,
				Pattern:   pattern,
			},
		},
	}); err != nil {
		return nil, errors.Wrap(err, "send file list request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("request cancelled")
		}
		fileResp := resp.GetFileListResponse()
		if fileResp == nil {
			return nil, errors.New("unexpected response type")
		}

		return fileResp, nil
	}
}

func (s *Service) RequestFileOperation(
	ctx context.Context,
	nodeID uint64,
	req *proto.FileOperationRequest,
) (*proto.FileOperationResponse, error) {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return nil, errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	req.RequestId = requestID

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_FileOperation{
			FileOperation: req,
		},
	}); err != nil {
		return nil, errors.Wrap(err, "send file operation request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("request cancelled")
		}
		opResp := resp.GetFileOperationResponse()
		if opResp == nil {
			return nil, errors.New("unexpected response type")
		}

		return opResp, nil
	}
}

func (s *Service) RequestCommand(
	ctx context.Context,
	nodeID uint64,
	req *proto.CommandRequest,
) (*proto.CommandResult, error) {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return nil, errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_Command{
			Command: req,
		},
	}); err != nil {
		return nil, errors.Wrap(err, "send command request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("request cancelled")
		}
		cmdResult := resp.GetCommandResult()
		if cmdResult == nil {
			return nil, errors.New("unexpected response type")
		}

		return cmdResult, nil
	}
}

func (s *Service) RequestStatus(
	ctx context.Context,
	nodeID uint64,
) (*proto.StatusResponse, error) {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return nil, errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_StatusRequest{
			StatusRequest: &proto.StatusRequest{},
		},
	}); err != nil {
		return nil, errors.Wrap(err, "send status request")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return nil, errors.New("request cancelled")
		}
		statusResp := resp.GetStatusResponse()
		if statusResp == nil {
			return nil, errors.New("unexpected response type")
		}

		return statusResp, nil
	}
}

func (s *Service) RequestFileUploadTask(
	ctx context.Context,
	nodeID uint64,
	transferID, destPath, checksum string,
	totalSize int64,
) error {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_FileUploadTask{
			FileUploadTask: &proto.FileUploadTask{
				TransferId:     transferID,
				Path:           destPath,
				ChecksumSha256: checksum,
				TotalSize:      totalSize,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "send file upload task")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return errors.New("request cancelled")
		}
		fileResp := resp.GetFileWriteResponse()
		if fileResp == nil {
			return errors.New("unexpected response type")
		}
		if !fileResp.Success {
			return errors.New(fileResp.Error)
		}

		return nil
	}
}

func (s *Service) RequestFileDownloadTask(
	ctx context.Context,
	nodeID uint64,
	transferID, srcPath string,
) error {
	sess, ok := s.registry.GetSession(nodeID)
	if !ok {
		return errors.New("node not connected")
	}

	requestID := generateRequestID()
	respCh := sess.RegisterPendingRequest(requestID)
	defer sess.CancelPendingRequest(requestID)

	if err := sess.Send(&proto.GatewayMessage{
		RequestId: requestID,
		Payload: &proto.GatewayMessage_FileDownloadTask{
			FileDownloadTask: &proto.FileDownloadTask{
				TransferId: transferID,
				Path:       srcPath,
			},
		},
	}); err != nil {
		return errors.Wrap(err, "send file download task")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return errors.New("request cancelled")
		}
		fileResp := resp.GetFileWriteResponse()
		if fileResp == nil {
			return errors.New("unexpected response type")
		}
		if !fileResp.Success {
			return errors.New(fileResp.Error)
		}

		return nil
	}
}

func (s *Service) Enroll(ctx context.Context, req *proto.EnrollRequest) (*proto.EnrollResponse, error) {
	if s.enrollmentSvc == nil {
		return nil, status.Error(codes.Unavailable, "enrollment is not enabled")
	}

	host := req.Host
	if host == "" {
		if p, ok := peer.FromContext(ctx); ok {
			host = p.Addr.String()
		}
	}

	result, err := s.enrollmentSvc.Enroll(ctx, req.SetupKey, &enrollment.EnrollInput{
		Host:         host,
		Port:         req.Port,
		OS:           req.Os,
		Version:      req.Version,
		Capabilities: req.Capabilities,
	})
	if err != nil {
		if errors.Is(err, enrollment.ErrInvalidSetupKey) || errors.Is(err, enrollment.ErrSetupKeyNotConfigured) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		s.logger.Error("enrollment failed", "error", err)

		return nil, status.Error(codes.Internal, "enrollment failed")
	}

	s.logger.Info("daemon enrolled",
		"node_id", result.NodeID,
		"host", host,
		"os", req.Os,
		"version", req.Version,
	)

	return &proto.EnrollResponse{
		Success:           true,
		NodeId:            uint64(result.NodeID),
		ApiKey:            result.APIKey,
		RootCertificate:   result.RootCertificate,
		ServerCertificate: result.ServerCertificate,
		ServerPrivateKey:  result.ServerPrivateKey,
	}, nil
}

func generateRequestID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)[:n]
}
