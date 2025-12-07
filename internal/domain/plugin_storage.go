package domain

import "time"

type PluginStorageEntry struct {
	ID         uint64     `db:"id"`
	PluginID   uint64     `db:"plugin_id"`
	Key        string     `db:"key"`
	EntityType *string    `db:"entity_type"`
	EntityID   *uint      `db:"entity_id"`
	Payload    []byte     `db:"payload"`
	CreatedAt  *time.Time `db:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at"`
}

type PluginStorageEntityPair struct {
	EntityType *string
	EntityID   *uint
}
