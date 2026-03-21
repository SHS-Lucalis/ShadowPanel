package interceptors

import (
	"context"
	"crypto/subtle"
	"log/slog"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	NodeIDKey         contextKey = "node_id"
	NodeKey           contextKey = "node"
	APIKeyMetadataKey            = "x-api-key"
	NodeIDMetadataKey            = "x-node-id"
)

type AuthInterceptor struct {
	nodeRepo    repositories.NodeRepository
	requireMTLS bool
	logger      *slog.Logger
}

func NewAuthInterceptor(
	nodeRepo repositories.NodeRepository,
	requireMTLS bool,
	logger *slog.Logger,
) *AuthInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuthInterceptor{
		nodeRepo:    nodeRepo,
		requireMTLS: requireMTLS,
		logger:      logger,
	}
}

func (i *AuthInterceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		if i.requireMTLS {
			if err := i.verifyMTLS(ctx); err != nil {
				return err
			}
		}

		return handler(srv, ss)
	}
}

func (i *AuthInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		if i.requireMTLS {
			if err := i.verifyMTLS(ctx); err != nil {
				return nil, err
			}
		}

		nodeID, err := i.extractAndVerifyAPIKey(ctx)
		if err != nil {
			return nil, err
		}

		if nodeID > 0 {
			ctx = context.WithValue(ctx, NodeIDKey, nodeID)
		}

		return handler(ctx, req)
	}
}

func (i *AuthInterceptor) verifyMTLS(ctx context.Context) error {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "no peer information")
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return status.Error(codes.Unauthenticated, "no TLS info")
	}

	if len(tlsInfo.State.PeerCertificates) == 0 {
		return status.Error(codes.Unauthenticated, "no client certificate")
	}

	return nil
}

func (i *AuthInterceptor) extractAndVerifyAPIKey(ctx context.Context) (uint64, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, nil
	}

	apiKeys := md.Get(APIKeyMetadataKey)
	if len(apiKeys) == 0 {
		return 0, nil
	}

	nodeIDs := md.Get(NodeIDMetadataKey)
	if len(nodeIDs) == 0 {
		return 0, status.Error(codes.InvalidArgument, "node ID required with API key")
	}

	var nodeID uint64
	for _, c := range nodeIDs[0] {
		if c >= '0' && c <= '9' {
			nodeID = nodeID*10 + uint64(c-'0')
		} else {
			return 0, status.Error(codes.InvalidArgument, "invalid node ID format")
		}
	}

	nodes, err := i.nodeRepo.Find(ctx, &filters.FindNode{IDs: []uint{uint(nodeID)}}, nil, nil)
	if err != nil {
		i.logger.Error("failed to find node", "node_id", nodeID, "error", err)
		return 0, status.Error(codes.Internal, "failed to verify node")
	}

	if len(nodes) == 0 {
		return 0, status.Error(codes.NotFound, "node not found")
	}

	node := nodes[0]

	if !node.Enabled {
		return 0, status.Error(codes.PermissionDenied, "node is disabled")
	}

	if !secureCompare(apiKeys[0], node.GdaemonAPIKey) {
		return 0, status.Error(codes.Unauthenticated, "invalid API key")
	}

	return nodeID, nil
}

func secureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func GetNodeIDFromContext(ctx context.Context) (uint64, bool) {
	v := ctx.Value(NodeIDKey)
	if v == nil {
		return 0, false
	}
	nodeID, ok := v.(uint64)
	return nodeID, ok
}

func GetNodeFromContext(ctx context.Context) (*domain.Node, bool) {
	v := ctx.Value(NodeKey)
	if v == nil {
		return nil, false
	}
	node, ok := v.(*domain.Node)
	return node, ok
}
