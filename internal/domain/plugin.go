package domain

import "time"

type PluginStatus string

const (
	PluginStatusActive   PluginStatus = "active"
	PluginStatusDisabled PluginStatus = "disabled"
	PluginStatusError    PluginStatus = "error"
	PluginStatusUpdating PluginStatus = "updating"
)

type Plugin struct {
	ID                  uint               `db:"id"`
	Name                string             `db:"name"`
	Version             string             `db:"version"`
	Description         string             `db:"description"`
	Author              string             `db:"author"`
	APIVersion          string             `db:"api_version"`
	Filename            *string            `db:"filename"`
	Source              *string            `db:"source"`
	Homepage            *string            `db:"homepage"`
	RequiredPermissions []PluginPermission `db:"-"`
	AllowedPermissions  []PluginPermission `db:"-"`
	Status              PluginStatus       `db:"status"`
	Priority            int                `db:"priority"`
	Category            *string            `db:"category"`
	Dependencies        []string           `db:"-"`
	Config              map[string]any     `db:"-"`
	InstalledAt         *time.Time         `db:"installed_at"`
	LastLoadedAt        *time.Time         `db:"last_loaded_at"`
	CreatedAt           *time.Time         `db:"created_at"`
	UpdatedAt           *time.Time         `db:"updated_at"`
}

type PluginPermission string

const (
	PluginPermissionManageServers  PluginPermission = "manage_servers"
	PluginPermissionManageNodes    PluginPermission = "manage_nodes"
	PluginPermissionManageGames    PluginPermission = "manage_games"
	PluginPermissionManageGameMods PluginPermission = "manage_game_mods"
	PluginPermissionManageUsers    PluginPermission = "manage_users"
	PluginPermissionFiles          PluginPermission = "files"
	PluginPermissionListenEvents   PluginPermission = "listen_events"
)
