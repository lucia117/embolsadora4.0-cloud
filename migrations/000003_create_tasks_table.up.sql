-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Índice para búsquedas por título
CREATE INDEX IF NOT EXISTS idx_tasks_title ON tasks (title);

-- Índice para filtrar por estado de completado
CREATE INDEX IF NOT EXISTS idx_tasks_completed ON tasks (completed);
-- +goose StatementEnd
