-- +goose Up
ALTER TABLE servers CHANGE COLUMN uuid uid varchar(36) NOT NULL;
ALTER TABLE servers DROP COLUMN uuid_short;

-- +goose Down
ALTER TABLE servers CHANGE COLUMN uid uuid varchar(36) NOT NULL;
ALTER TABLE servers ADD COLUMN uuid_short varchar(8) NOT NULL DEFAULT '';
UPDATE servers SET uuid_short = LEFT(uuid, 8);
