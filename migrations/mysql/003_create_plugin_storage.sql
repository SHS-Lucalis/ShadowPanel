-- +goose Up

CREATE TABLE plugin_storage (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
    plugin_id BIGINT UNSIGNED NOT NULL,
    `key` VARCHAR(255) NOT NULL,
    entity_type VARCHAR(128) DEFAULT NULL,
    entity_id INT UNSIGNED DEFAULT NULL,
    payload BLOB NOT NULL,
    created_at TIMESTAMP DEFAULT NULL,
    updated_at TIMESTAMP DEFAULT NULL,
    UNIQUE KEY plugin_storage_lookup_index (plugin_id, `key`, entity_type, entity_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- +goose Down

DROP TABLE IF EXISTS plugin_storage;
