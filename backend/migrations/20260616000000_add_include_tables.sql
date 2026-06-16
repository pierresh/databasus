-- +goose Up
ALTER TABLE mariadb_databases
    ADD COLUMN include_tables TEXT NOT NULL DEFAULT '';

ALTER TABLE mysql_databases
    ADD COLUMN include_tables TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE mariadb_databases
    DROP COLUMN include_tables;

ALTER TABLE mysql_databases
    DROP COLUMN include_tables;
