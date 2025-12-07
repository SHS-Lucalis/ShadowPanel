-- +goose Up

CREATE TABLE plugin_storage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    plugin_id INTEGER NOT NULL,
    key TEXT NOT NULL,
    entity_type TEXT DEFAULT NULL,
    entity_id INTEGER DEFAULT NULL,
    payload BLOB NOT NULL,
    created_at TEXT DEFAULT NULL,
    updated_at TEXT DEFAULT NULL
);

CREATE UNIQUE INDEX plugin_storage_lookup_index ON plugin_storage (plugin_id, key, entity_type, entity_id);

-- +goose Down

DROP TABLE IF EXISTS plugin_storage;
