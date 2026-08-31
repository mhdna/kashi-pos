-- name: ListLoanCategories :many
SELECT * FROM loan_categories ORDER BY name;

-- name: ListActiveLoanCategories :many
SELECT * FROM loan_categories WHERE is_active = 1 ORDER BY name;

-- name: GetLoanCategoryByName :one
SELECT * FROM loan_categories WHERE name = ?;

-- name: CreateLoanCategory :one
INSERT INTO loan_categories (name, is_active, icon, color)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: UpdateLoanCategory :one
UPDATE loan_categories
SET name = ?, is_active = ?, icon = ?, color = ?, updated_at = datetime('now')
WHERE id = ?
RETURNING *;
