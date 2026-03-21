package channels

import "strconv"

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
	return DaemonTaskDispatch + formatUint64(nodeID)
}

func BuildDaemonCommandDispatchChannel(nodeID uint64) string {
	return DaemonCommandDispatch + formatUint64(nodeID)
}

func formatUint64(n uint64) string {
	return strconv.FormatUint(n, 10)
}
