-- +goose Up

CREATE TABLE plugins (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    version TEXT NOT NULL,
    description TEXT NOT NULL,
    author TEXT NOT NULL,
    api_version TEXT NOT NULL,
    filename TEXT DEFAULT NULL,
    source TEXT DEFAULT NULL,
    homepage TEXT DEFAULT NULL,
    required_permissions TEXT DEFAULT NULL,
    allowed_permissions TEXT DEFAULT NULL,
    status TEXT NOT NULL DEFAULT 'disabled',
    priority INTEGER NOT NULL DEFAULT 0,
    category TEXT DEFAULT NULL,
    dependencies TEXT DEFAULT NULL,
    config TEXT DEFAULT NULL,
    installed_at TEXT DEFAULT NULL,
    last_loaded_at TEXT DEFAULT NULL,
    created_at TEXT DEFAULT NULL,
    updated_at TEXT DEFAULT NULL
);

-- +goose Down

DROP TABLE IF EXISTS plugins;
