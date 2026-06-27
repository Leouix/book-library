package storage

import (
	"context"
)

// User represents a user record in the database.
type User struct {
	ID           int32
	Username     string
	PasswordHash string
}

const createUser = `INSERT INTO users (username, password_hash)
VALUES ($1, $2)
RETURNING id, username, password_hash`

// CreateUser inserts a new user and returns the created record.
func (q *Queries) CreateUser(ctx context.Context, username, passwordHash string) (User, error) {
	row := q.db.QueryRow(ctx, createUser, username, passwordHash)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash)
	return u, err
}

const getUserByUsername = `SELECT id, username, password_hash
FROM users
WHERE username = $1`

// GetUserByUsername looks up a user by username.
func (q *Queries) GetUserByUsername(ctx context.Context, username string) (User, error) {
	row := q.db.QueryRow(ctx, getUserByUsername, username)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash)
	return u, err
}
