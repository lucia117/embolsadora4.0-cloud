-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_tasks_title;
DROP INDEX IF EXISTS idx_tasks_completed;
DROP TABLE IF EXISTS tasks;
-- +goose StatementEnd
