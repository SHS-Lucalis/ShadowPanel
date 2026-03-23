package gateway

import (
	"encoding/json"
	"fmt"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func domainServerToProto(srv *domain.Server) *proto.Server {
	var createdAt, updatedAt, deletedAt, expires, lastProcessCheck *int64

	if srv.CreatedAt != nil {
		createdAt = lo.ToPtr(srv.CreatedAt.Unix())
	}
	if srv.UpdatedAt != nil {
		updatedAt = lo.ToPtr(srv.UpdatedAt.Unix())
	}
	if srv.DeletedAt != nil {
		deletedAt = lo.ToPtr(srv.DeletedAt.Unix())
	}
	if srv.Expires != nil {
		expires = lo.ToPtr(srv.Expires.Unix())
	}
	if srv.LastProcessCheck != nil {
		lastProcessCheck = lo.ToPtr(srv.LastProcessCheck.Unix())
	}

	var queryPort, rconPort *int32
	if srv.QueryPort != nil {
		queryPort = lo.ToPtr(int32(*srv.QueryPort))
	}
	if srv.RconPort != nil {
		rconPort = lo.ToPtr(int32(*srv.RconPort))
	}

	var cpuLimit, ramLimit, netLimit *int32
	if srv.CPULimit != nil {
		cpuLimit = lo.ToPtr(int32(*srv.CPULimit))
	}
	if srv.RAMLimit != nil {
		ramLimit = lo.ToPtr(int32(*srv.RAMLimit))
	}
	if srv.NetLimit != nil {
		netLimit = lo.ToPtr(int32(*srv.NetLimit))
	}

	var varsStr *string
	if srv.Vars != nil {
		varsBytes, err := json.Marshal(srv.Vars)
		if err == nil {
			varsStr = lo.ToPtr(string(varsBytes))
		}
	}

	return &proto.Server{
		Id:               uint64(srv.ID),
		Uuid:             srv.UUID.String(),
		UuidShort:        srv.UUIDShort,
		Enabled:          srv.Enabled,
		Installed:        domainInstalledStatusToProto(srv.Installed),
		Blocked:          srv.Blocked,
		Name:             srv.Name,
		GameId:           srv.GameID,
		DsId:             uint64(srv.DSID),
		GameModId:        uint64(srv.GameModID),
		Expires:          expires,
		ServerIp:         srv.ServerIP,
		ServerPort:       int32(srv.ServerPort),
		QueryPort:        queryPort,
		RconPort:         rconPort,
		Rcon:             srv.Rcon,
		Dir:              srv.Dir,
		SuUser:           srv.SuUser,
		CpuLimit:         cpuLimit,
		RamLimit:         ramLimit,
		NetLimit:         netLimit,
		StartCommand:     srv.StartCommand,
		StopCommand:      srv.StopCommand,
		ForceStopCommand: srv.ForceStopCommand,
		RestartCommand:   srv.RestartCommand,
		ProcessActive:    srv.ProcessActive,
		LastProcessCheck: lastProcessCheck,
		Vars:             varsStr,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		DeletedAt:        deletedAt,
		Metadata:         domainMetadataToProto(srv.Metadata),
	}
}

func domainInstalledStatusToProto(status domain.ServerInstalledStatus) proto.ServerInstalledStatus {
	switch status {
	case domain.ServerInstalledStatusNotInstalled:
		return proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_NOT_INSTALLED
	case domain.ServerInstalledStatusInstalled:
		return proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_INSTALLED
	case domain.ServerInstalledStatusInstallationInProg:
		return proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_INSTALLATION_IN_PROGRESS
	default:
		return proto.ServerInstalledStatus_SERVER_INSTALLED_STATUS_NOT_INSTALLED
	}
}

func domainGameToProto(g *domain.Game) *proto.Game {
	var steamAppIDLinux, steamAppIDWindows *uint32
	if g.SteamAppIDLinux != nil {
		steamAppIDLinux = lo.ToPtr(uint32(*g.SteamAppIDLinux))
	}
	if g.SteamAppIDWindows != nil {
		steamAppIDWindows = lo.ToPtr(uint32(*g.SteamAppIDWindows))
	}

	return &proto.Game{
		Code:                    g.Code,
		Name:                    g.Name,
		Engine:                  g.Engine,
		EngineVersion:           g.EngineVersion,
		SteamAppIdLinux:         steamAppIDLinux,
		SteamAppIdWindows:       steamAppIDWindows,
		SteamAppSetConfig:       g.SteamAppSetConfig,
		RemoteRepositoryLinux:   g.RemoteRepositoryLinux,
		RemoteRepositoryWindows: g.RemoteRepositoryWindows,
		Enabled:                 g.Enabled != 0,
		LocalRepositoryLinux:    g.LocalRepositoryLinux,
		LocalRepositoryWindows:  g.LocalRepositoryWindows,
		Metadata:                domainMetadataToProto(g.Metadata),
	}
}

func domainGameModToProto(gm *domain.GameMod) *proto.GameMod {
	fastRcon := make([]*proto.GameModFastRcon, 0, len(gm.FastRcon))
	for _, fr := range gm.FastRcon {
		fastRcon = append(fastRcon, &proto.GameModFastRcon{
			Info:    fr.Info,
			Command: fr.Command,
		})
	}

	vars := make([]*proto.GameModVar, 0, len(gm.Vars))
	for _, v := range gm.Vars {
		vars = append(vars, &proto.GameModVar{
			Var:      v.Var,
			Default:  string(v.Default),
			Info:     v.Info,
			AdminVar: v.AdminVar,
		})
	}

	return &proto.GameMod{
		Id:                      uint64(gm.ID),
		GameCode:                gm.GameCode,
		Name:                    gm.Name,
		StartCmdLinux:           gm.StartCmdLinux,
		StartCmdWindows:         gm.StartCmdWindows,
		KickCmd:                 gm.KickCmd,
		BanCmd:                  gm.BanCmd,
		FastRcon:                fastRcon,
		Vars:                    vars,
		RemoteRepositoryLinux:   gm.RemoteRepositoryLinux,
		RemoteRepositoryWindows: gm.RemoteRepositoryWindows,
		LocalRepositoryLinux:    gm.LocalRepositoryLinux,
		LocalRepositoryWindows:  gm.LocalRepositoryWindows,
		ChnameCmd:               gm.ChnameCmd,
		SrestartCmd:             gm.SrestartCmd,
		ChmapCmd:                gm.ChmapCmd,
		SendmsgCmd:              gm.SendmsgCmd,
		PasswdCmd:               gm.PasswdCmd,
		Metadata:                domainMetadataToProto(gm.Metadata),
	}
}

func domainMetadataToProto(metadata domain.Metadata) map[string]*anypb.Any {
	if metadata == nil {
		return nil
	}

	result := make(map[string]*anypb.Any, len(metadata))
	for k, v := range metadata {
		anyVal, err := anypb.New(wrapperspb.String(fmt.Sprint(v)))
		if err != nil {
			continue
		}
		result[k] = anyVal
	}

	return result
}

func DomainDaemonTaskToProto(task *domain.DaemonTask) *proto.DaemonTask {
	var createdAt, updatedAt *int64
	if task.CreatedAt != nil {
		createdAt = lo.ToPtr(task.CreatedAt.Unix())
	}
	if task.UpdatedAt != nil {
		updatedAt = lo.ToPtr(task.UpdatedAt.Unix())
	}

	var serverID *uint64
	if task.ServerID != nil {
		serverID = lo.ToPtr(uint64(*task.ServerID))
	}

	var runAfterID *uint64
	if task.RunAftID != nil {
		runAfterID = lo.ToPtr(uint64(*task.RunAftID))
	}

	return &proto.DaemonTask{
		Id:         uint64(task.ID),
		RunAfterId: runAfterID,
		NodeId:     uint64(task.DedicatedServerID),
		ServerId:   serverID,
		TaskType:   domainTaskTypeToProto(task.Task),
		Data:       task.Data,
		Cmd:        task.Cmd,
		Output:     task.Output,
		Status:     domainTaskStatusToProto(task.Status),
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

func domainTaskTypeToProto(taskType domain.DaemonTaskType) proto.DaemonTaskType {
	switch taskType {
	case domain.DaemonTaskTypeServerStart:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_START
	case domain.DaemonTaskTypeServerStop:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_STOP
	case domain.DaemonTaskTypeServerRestart:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_RESTART
	case domain.DaemonTaskTypeServerUpdate:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_UPDATE
	case domain.DaemonTaskTypeServerInstall:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_INSTALL
	case domain.DaemonTaskTypeServerDelete:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_DELETE
	case domain.DaemonTaskTypeServerMove:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_SERVER_MOVE
	case domain.DaemonTaskTypeCmdExec:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_CMD_EXEC
	default:
		return proto.DaemonTaskType_DAEMON_TASK_TYPE_UNSPECIFIED
	}
}

func domainTaskStatusToProto(status domain.DaemonTaskStatus) proto.DaemonTaskStatus {
	switch status {
	case domain.DaemonTaskStatusWaiting:
		return proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WAITING
	case domain.DaemonTaskStatusWorking:
		return proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WORKING
	case domain.DaemonTaskStatusError:
		return proto.DaemonTaskStatus_DAEMON_TASK_STATUS_ERROR
	case domain.DaemonTaskStatusSuccess:
		return proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS
	case domain.DaemonTaskStatusCanceled:
		return proto.DaemonTaskStatus_DAEMON_TASK_STATUS_CANCELED
	default:
		return proto.DaemonTaskStatus_DAEMON_TASK_STATUS_UNSPECIFIED
	}
}

func ProtoTaskStatusToDomain(status proto.DaemonTaskStatus) domain.DaemonTaskStatus {
	switch status {
	case proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WAITING:
		return domain.DaemonTaskStatusWaiting
	case proto.DaemonTaskStatus_DAEMON_TASK_STATUS_WORKING:
		return domain.DaemonTaskStatusWorking
	case proto.DaemonTaskStatus_DAEMON_TASK_STATUS_ERROR:
		return domain.DaemonTaskStatusError
	case proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS:
		return domain.DaemonTaskStatusSuccess
	case proto.DaemonTaskStatus_DAEMON_TASK_STATUS_CANCELED:
		return domain.DaemonTaskStatusCanceled
	default:
		return domain.DaemonTaskStatusWaiting
	}
}
