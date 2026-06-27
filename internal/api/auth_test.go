package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"

	"book-library/internal/storage"
)

func TestRegister(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       any
		mock       *mockUserStore
		wantStatus int
		wantUser   string
	}{
		{
			name: "success",
			body: map[string]string{"username": "alice", "password": "pass"},
			mock: &mockUserStore{
				createUserFn: func(_ context.Context, arg storage.CreateUserParams) (storage.User, error) {
					return storage.User{ID: 1, Username: arg.Username, PasswordHash: arg.PasswordHash}, nil
				},
			},
			wantStatus: http.StatusCreated,
			wantUser:   "alice",
		},
		{
			name: "duplicate username",
			body: map[string]string{"username": "alice", "password": "pass"},
			mock: &mockUserStore{
				createUserFn: func(_ context.Context, _ storage.CreateUserParams) (storage.User, error) {
					return storage.User{}, pgx.ErrNoRows
				},
			},
			wantStatus: http.StatusConflict,
		},
		{
			name:       "invalid JSON",
			body:       "<<<>>>",
			mock:       &mockUserStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty fields",
			body:       map[string]string{"username": "", "password": ""},
			mock:       &mockUserStore{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(tt.body); err != nil {
				t.Fatal(err)
			}

			h := NewHandler(&mockBookStore{}, tt.mock, []byte("secret"))
			req := httptest.NewRequest(http.MethodPost, "/register", &buf)
			rec := httptest.NewRecorder()

			h.Register(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantUser != "" {
				var resp authResponse
				decodeBody(t, rec.Body, &resp)
				if resp.Username != tt.wantUser {
					t.Errorf("username = %q, want %q", resp.Username, tt.wantUser)
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	t.Parallel()

	validHash, err := bcrypt.GenerateFromPassword([]byte("pass"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		body       any
		mock       *mockUserStore
		wantStatus int
		wantToken  bool
	}{
		{
			name: "success",
			body: map[string]string{"username": "alice", "password": "pass"},
			mock: &mockUserStore{
				getUserByUsernameFn: func(_ context.Context, username string) (storage.User, error) {
					return storage.User{ID: 1, Username: username, PasswordHash: string(validHash)}, nil
				},
			},
			wantStatus: http.StatusOK,
			wantToken:  true,
		},
		{
			name: "wrong password",
			body: map[string]string{"username": "alice", "password": "wrong"},
			mock: &mockUserStore{
				getUserByUsernameFn: func(_ context.Context, username string) (storage.User, error) {
					return storage.User{ID: 1, Username: username, PasswordHash: string(validHash)}, nil
				},
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "user not found",
			body: map[string]string{"username": "alice", "password": "pass"},
			mock: &mockUserStore{
				getUserByUsernameFn: func(_ context.Context, _ string) (storage.User, error) {
					return storage.User{}, pgx.ErrNoRows
				},
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid JSON",
			body:       "<<<>>>",
			mock:       &mockUserStore{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(tt.body); err != nil {
				t.Fatal(err)
			}

			h := NewHandler(&mockBookStore{}, tt.mock, []byte("secret"))
			req := httptest.NewRequest(http.MethodPost, "/login", &buf)
			rec := httptest.NewRecorder()

			h.Login(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantToken {
				var resp authResponse
				decodeBody(t, rec.Body, &resp)
				if resp.Token == "" {
					t.Error("expected a token")
				}
				if resp.Username != "alice" {
					t.Errorf("username = %q, want %q", resp.Username, "alice")
				}
			}
		})
	}
}

func TestAuthMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		header     string
		wantStatus int
	}{
		{
			name:       "no authorization header",
			header:     "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "malformed header",
			header:     "Basic token",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "invalid token",
			header:     "Bearer invalidtoken",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "valid token",
			header:     "Bearer " + makeToken(t, []byte("secret"), "alice"),
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(&mockBookStore{}, &mockUserStore{}, []byte("secret"))

			var called bool
			next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				u, ok := UsernameFromContext(r.Context())
				if !ok || u != "alice" {
					t.Error("username not set in context")
				}
			})

			handler := h.AuthMiddleware(next)
			req := httptest.NewRequest(http.MethodPost, "/books", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK && !called {
				t.Error("next handler was not called")
			}
		})
	}
}
