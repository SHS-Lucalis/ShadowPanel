-- +goose Up

CREATE TABLE plugins (
    id INT UNSIGNED NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(64) NOT NULL,
    description TEXT NOT NULL,
    author VARCHAR(255) NOT NULL,
    api_version VARCHAR(32) NOT NULL,
    filename VARCHAR(255) DEFAULT NULL,
    source VARCHAR(512) DEFAULT NULL,
    homepage VARCHAR(512) DEFAULT NULL,
    required_permissions TEXT DEFAULT NULL,
    allowed_permissions TEXT DEFAULT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'disabled',
    priority INT NOT NULL DEFAULT 0,
    category VARCHAR(128) DEFAULT NULL,
    dependencies TEXT DEFAULT NULL,
    config TEXT DEFAULT NULL,
    installed_at TIMESTAMP DEFAULT NULL,
    last_loaded_at TIMESTAMP DEFAULT NULL,
    created_at TIMESTAMP DEFAULT NULL,
    updated_at TIMESTAMP DEFAULT NULL,
    UNIQUE KEY plugins_name_unique (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE IF EXISTS plugins;
