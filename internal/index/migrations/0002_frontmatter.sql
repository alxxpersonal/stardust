-- +goose Up
-- +goose StatementBegin
ALTER TABLE catalog ADD COLUMN frontmatter TEXT NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE catalog DROP COLUMN frontmatter;
-- +goose StatementEnd
