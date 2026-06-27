-- name: CreateBook :one
INSERT INTO books (title, author, year, file_url, s3_key, file_name)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, title, author, year, file_url, s3_key, file_name;

-- name: GetBook :one
SELECT id, title, author, year, file_url, s3_key, file_name
FROM books
WHERE id = $1;

-- name: ListBooks :many
SELECT id, title, author, year, file_url, s3_key, file_name
FROM books
ORDER BY id;

-- name: UpdateBook :one
UPDATE books
SET title = $2, author = $3, year = $4
WHERE id = $1
RETURNING id, title, author, year, file_url, s3_key, file_name;

-- name: DeleteBook :exec
DELETE FROM books
WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES ($1, $2)
RETURNING id, username, password_hash;

-- name: GetUserByUsername :one
SELECT id, username, password_hash
FROM users
WHERE username = $1;
