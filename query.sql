-- name: CreateBook :one
INSERT INTO books (title, author, year)
VALUES ($1, $2, $3)
RETURNING id, title, author, year;

-- name: GetBook :one
SELECT id, title, author, year
FROM books
WHERE id = $1;

-- name: ListBooks :many
SELECT id, title, author, year
FROM books
ORDER BY id;

-- name: UpdateBook :one
UPDATE books
SET title = $2, author = $3, year = $4
WHERE id = $1
RETURNING id, title, author, year;

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

-- name: CreateFile :one
INSERT INTO files (original_name, s3_key, mime_type, size)
VALUES ($1, $2, $3, $4)
RETURNING id, original_name, s3_key, mime_type, size, created_at;

-- name: GetFile :one
SELECT id, original_name, s3_key, mime_type, size, created_at
FROM files
WHERE id = $1;
