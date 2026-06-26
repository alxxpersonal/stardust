-- +goose Up
-- +goose StatementBegin
ALTER TABLE chunks ADD COLUMN chunk_hash TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE chunks DROP COLUMN chunk_hash;
-- +goose StatementEnd
