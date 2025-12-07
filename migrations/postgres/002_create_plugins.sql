-- +goose Up

CREATE TABLE plugins (
    id INTEGER PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    version VARCHAR(64) NOT NULL,
    description TEXT NOT NULL,
    author VARCHAR(255) NOT NULL,
    api_version VARCHAR(32) NOT NULL,
    filename VARCHAR(255) DEFAULT NULL,
    source VARCHAR(512) DEFAULT NULL,
    homepage VARCHAR(512) DEFAULT NULL,
    required_permissions TEXT[] DEFAULT NULL,
    allowed_permissions TEXT[] DEFAULT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'disabled',
    priority INTEGER NOT NULL DEFAULT 0,
    category VARCHAR(128) DEFAULT NULL,
    dependencies TEXT[] DEFAULT NULL,
    config JSONB DEFAULT NULL,
    installed_at TIMESTAMPTZ DEFAULT NULL,
    last_loaded_at TIMESTAMPTZ DEFAULT NULL,
    created_at TIMESTAMPTZ DEFAULT NULL,
    updated_at TIMESTAMPTZ DEFAULT NULL
);

-- +goose Down

DROP TABLE IF EXISTS plugins;
