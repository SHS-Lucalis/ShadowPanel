package channels

import (
	"strconv"
	"strings"
)

const (
	Prefix = "gameap:"

	CachePrefix          = Prefix + "cache:"
	CacheInvalidate      = CachePrefix + "invalidate"
	CacheInvalidateGames = CachePrefix + "invalidate:games"
	CacheInvalidateUsers = CachePrefix + "invalidate:users"
	CacheInvalidateNodes = CachePrefix + "invalidate:nodes"
	CacheInvalidateRBAC  = CachePrefix + "invalidate:rbac"
	CacheInvalidateAll   = CachePrefix + "invalidate:*"

	PluginPrefix       = Prefix + "plugin:"
	PluginEvents       = PluginPrefix + "events"
	PluginServerEvents = PluginPrefix + "events:server"
	PluginTaskEvents   = PluginPrefix + "events:task"

	RealtimePrefix        = Prefix + "realtime:"
	RealtimeServerStatus  = RealtimePrefix + "server:status"
	RealtimeTaskProgress  = RealtimePrefix + "task:progress"
	RealtimeNotifications = RealtimePrefix + "notifications"

	RealtimeTaskStatus    = RealtimePrefix + "task:status:"
	RealtimeTaskOutput    = RealtimePrefix + "task:output:"
	RealtimeTaskAll       = RealtimePrefix + "task:*"
	RealtimeConsoleOutput = RealtimePrefix + "console:output:"
	RealtimeConsoleResult = RealtimePrefix + "console:result:"
	RealtimeConsoleAll    = RealtimePrefix + "console:*"

	SystemPrefix       = Prefix + "system:"
	SystemShutdown     = SystemPrefix + "shutdown"
	SystemConfigReload = SystemPrefix + "config:reload"

	DaemonPrefix           = Prefix + "daemon:"
	DaemonSessionConnected = DaemonPrefix + "session:connected"
	DaemonSessionClosed    = DaemonPrefix + "session:closed"
	DaemonTaskDispatch     = DaemonPrefix + "task:dispatch:"
	DaemonCommandDispatch  = DaemonPrefix + "command:dispatch:"
	DaemonSessionAll       = DaemonPrefix + "session:*"
	DaemonTaskDispatchAll  = DaemonPrefix + "task:dispatch:*"
)

func BuildCacheInvalidateChannel(entityType string, entityID string) string {
	if entityID == "" {
		return CachePrefix + "invalidate:" + entityType
	}

	return CachePrefix + "invalidate:" + entityType + ":" + entityID
}

func BuildPluginEventChannel(eventType string) string {
	return PluginPrefix + "events:" + eventType
}

func BuildDaemonTaskDispatchChannel(nodeID uint64) string {
	sb := strings.Builder{}
	sb.Grow(len(DaemonTaskDispatch) + 20)

	sb.WriteString(DaemonTaskDispatch)
	sb.WriteString(strconv.FormatUint(nodeID, 10))

	return sb.String()
}

func BuildDaemonCommandDispatchChannel(nodeID uint64) string {
	sb := strings.Builder{}
	sb.Grow(len(DaemonCommandDispatch) + 20)

	sb.WriteString(DaemonCommandDispatch)
	sb.WriteString(strconv.FormatUint(nodeID, 10))

	return sb.String()
}

func BuildRealtimeTaskStatusChannel(taskID uint64) string {
	sb := strings.Builder{}
	sb.Grow(len(RealtimeTaskStatus) + 20)

	sb.WriteString(RealtimeTaskStatus)
	sb.WriteString(strconv.FormatUint(taskID, 10))

	return sb.String()
}

func BuildRealtimeTaskOutputChannel(taskID uint64) string {
	sb := strings.Builder{}
	sb.Grow(len(RealtimeTaskOutput) + 20)

	sb.WriteString(RealtimeTaskOutput)
	sb.WriteString(strconv.FormatUint(taskID, 10))

	return sb.String()
}

func BuildRealtimeConsoleOutputChannel(serverID uint64) string {
	sb := strings.Builder{}
	sb.Grow(len(RealtimeConsoleOutput) + 20)

	sb.WriteString(RealtimeConsoleOutput)
	sb.WriteString(strconv.FormatUint(serverID, 10))

	return sb.String()
}

func BuildRealtimeConsoleResultChannel(serverID uint64) string {
	sb := strings.Builder{}
	sb.Grow(len(RealtimeConsoleResult) + 20)

	sb.WriteString(RealtimeConsoleResult)
	sb.WriteString(strconv.FormatUint(serverID, 10))

	return sb.String()
}
