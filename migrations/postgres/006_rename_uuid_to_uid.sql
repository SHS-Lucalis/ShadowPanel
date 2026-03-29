-- +goose Up
ALTER TABLE servers RENAME COLUMN uuid TO uid;
ALTER TABLE servers DROP COLUMN uuid_short;

-- +goose Down
ALTER TABLE servers RENAME COLUMN uid TO uuid;
ALTER TABLE servers ADD COLUMN uuid_short VARCHAR(8) NOT NULL DEFAULT '';
UPDATE servers SET uuid_short = LEFT(uuid::TEXT, 8);
