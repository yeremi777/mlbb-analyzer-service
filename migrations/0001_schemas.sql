-- +goose Up
CREATE SCHEMA raw;
CREATE SCHEMA staging;
CREATE SCHEMA marts;

-- +goose Down
DROP SCHEMA marts;
DROP SCHEMA staging;
DROP SCHEMA raw;
