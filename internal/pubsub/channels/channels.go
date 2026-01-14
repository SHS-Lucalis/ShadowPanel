package channels

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
