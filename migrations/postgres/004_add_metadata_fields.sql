-- +goose Up
ALTER TABLE games ADD COLUMN metadata JSONB DEFAULT NULL;
ALTER TABLE game_mods ADD COLUMN metadata JSONB DEFAULT NULL;
ALTER TABLE servers ADD COLUMN metadata JSONB DEFAULT NULL;

-- +goose Down
ALTER TABLE games DROP COLUMN metadata;
ALTER TABLE game_mods DROP COLUMN metadata;
ALTER TABLE servers DROP COLUMN metadata;
