-- name: CreatePendingBook :one
INSERT INTO books (title, author, year, status)
VALUES ($1, $2, $3, 'pending')
RETURNING id, title, author, year, file_url, s3_key, file_name, status;

-- name: CompleteBook :exec
UPDATE books
SET file_url = $2, s3_key = $3, file_name = $4, status = 'completed'
WHERE id = $1;

-- name: FailBook :exec
UPDATE books SET status = 'failed' WHERE id = $1;

-- name: GetBook :one
SELECT id, title, author, year, file_url, s3_key, file_name, status
FROM books
WHERE id = $1;

-- name: ListBooks :many
SELECT id, title, author, year, file_url, s3_key, file_name, status
FROM books
WHERE status = 'completed'
ORDER BY id;

-- name: GetPendingBooks :many
SELECT id, title, author, year, file_url, s3_key, file_name, status
FROM books
WHERE status = 'pending' OR status = 'processing';

-- name: UpdateBook :one
UPDATE books
SET title = $2, author = $3, year = $4
WHERE id = $1
RETURNING id, title, author, year, file_url, s3_key, file_name, status;

-- name: DeleteBook :exec
DELETE FROM books WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES ($1, $2)
RETURNING id, username, password_hash;

-- name: GetUserByUsername :one
SELECT id, username, password_hash
FROM users
WHERE username = $1;
