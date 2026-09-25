----------------

-- each query needs a name and a type (:one, :many, :exec),
-- sqlc turns it into a method on db.Queries (run make sqlc after changes)

----------------

-- name: ListTodos :many
SELECT * FROM todos
ORDER BY description ASC;

-- name: SearchTodos :many
SELECT * FROM todos
WHERE description LIKE '%' || ? || '%'
ORDER BY description ASC;

-- name: CreateTodo :one
INSERT INTO todos (
  description, completed
) VALUES (
  ?, ?
)
RETURNING *;

-- name: UpdateTodoDescription :one
UPDATE todos
set description = ?
WHERE id = ?
RETURNING *;

-- name: ToggleTodoCompleted :one
UPDATE todos
set completed = NOT completed
WHERE id = ?
RETURNING *;

-- name: DeleteTodo :exec
DELETE FROM todos
WHERE id = ?;