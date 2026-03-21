-- +goose Up
CREATE TABLE pubsub_dlq (
    id VARCHAR(255) PRIMARY KEY,
    channel VARCHAR(255) NOT NULL,
    original_message JSON NOT NULL,
    error TEXT NOT NULL,
    attempt_count INT NOT NULL DEFAULT 1,
    failed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed BOOLEAN NOT NULL DEFAULT FALSE,
    processed_at DATETIME DEFAULT NULL
);

CREATE INDEX pubsub_dlq_processed_idx ON pubsub_dlq (processed);
CREATE INDEX pubsub_dlq_failed_at_idx ON pubsub_dlq (failed_at);

-- +goose Down
DROP TABLE IF EXISTS pubsub_dlq;
