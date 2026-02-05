package tasks

const createTaskQuery = `
		INSERT INTO tasks (id, title, description, completed, created_at, updated_at)
		VALUES ($id, $title, $description, $completed, $created_at, $updated_at)
	`

const getAllTasksQuery = `
		SELECT id, title, description, completed, created_at, updated_at
		FROM tasks
		ORDER BY created_at DESC
	`

const getTaskByIDQuery = `
		SELECT id, title, description, completed, created_at, updated_at
		FROM tasks
		WHERE id = $id
	`

const updateTaskQuery = `
		UPDATE tasks
		SET title = $title, description = $description, completed = $completed, updated_at = $updated_at
		WHERE id = $id
	`

const deleteTaskQuery = `
		DELETE FROM tasks WHERE id = $id
	`
