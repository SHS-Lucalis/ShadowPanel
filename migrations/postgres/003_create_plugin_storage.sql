-- +goose Up

CREATE TABLE plugin_storage (
    id BIGSERIAL PRIMARY KEY,
    plugin_id BIGINT NOT NULL,
    key VARCHAR(255) NOT NULL,
    entity_type VARCHAR(128) DEFAULT NULL,
    entity_id INTEGER DEFAULT NULL,
    payload BYTEA NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NULL,
    updated_at TIMESTAMPTZ DEFAULT NULL
);

CREATE UNIQUE INDEX plugin_storage_lookup_index ON plugin_storage (plugin_id, key, entity_type, entity_id);

-- +goose Down

DROP TABLE IF EXISTS plugin_storage;
