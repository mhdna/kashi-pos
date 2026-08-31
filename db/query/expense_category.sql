-- name: ListExpenseCategories :many
SELECT * FROM expense_categories ORDER BY name;

-- name: ListActiveExpenseCategories :many
SELECT * FROM expense_categories WHERE is_active = 1 ORDER BY name;

-- name: GetExpenseCategoryByName :one
SELECT * FROM expense_categories WHERE name = ?;

-- name: CreateExpenseCategory :one
INSERT INTO expense_categories (name, is_active, icon, color)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateExpenseCategory :one
UPDATE expense_categories
SET name = ?, is_active = ?, icon = ?, color = ?, updated_at = datetime('now')
WHERE id = ?
RETURNING *;
