package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"book-library/internal/storage"
)

type mockBookStore struct {
	createBookFn func(context.Context, storage.CreateBookParams) (storage.Book, error)
	getBookFn    func(context.Context, int32) (storage.Book, error)
	listBooksFn  func(context.Context) ([]storage.Book, error)
	updateBookFn func(context.Context, storage.UpdateBookParams) (storage.Book, error)
	deleteBookFn func(context.Context, int32) error
}

func (m *mockBookStore) CreateBook(ctx context.Context, arg storage.CreateBookParams) (storage.Book, error) {
	if m.createBookFn == nil {
		panic("mockBookStore.CreateBook: function not set")
	}
	return m.createBookFn(ctx, arg)
}

func (m *mockBookStore) GetBook(ctx context.Context, id int32) (storage.Book, error) {
	if m.getBookFn == nil {
		panic("mockBookStore.GetBook: function not set")
	}
	return m.getBookFn(ctx, id)
}

func (m *mockBookStore) ListBooks(ctx context.Context) ([]storage.Book, error) {
	if m.listBooksFn == nil {
		panic("mockBookStore.ListBooks: function not set")
	}
	return m.listBooksFn(ctx)
}

func (m *mockBookStore) UpdateBook(ctx context.Context, arg storage.UpdateBookParams) (storage.Book, error) {
	if m.updateBookFn == nil {
		panic("mockBookStore.UpdateBook: function not set")
	}
	return m.updateBookFn(ctx, arg)
}

func (m *mockBookStore) DeleteBook(ctx context.Context, id int32) error {
	if m.deleteBookFn == nil {
		panic("mockBookStore.DeleteBook: function not set")
	}
	return m.deleteBookFn(ctx, id)
}

type mockUserStore struct {
	createUserFn        func(context.Context, storage.CreateUserParams) (storage.User, error)
	getUserByUsernameFn func(context.Context, string) (storage.User, error)
}

func (m *mockUserStore) CreateUser(ctx context.Context, arg storage.CreateUserParams) (storage.User, error) {
	if m.createUserFn == nil {
		panic("mockUserStore.CreateUser: function not set")
	}
	return m.createUserFn(ctx, arg)
}

func (m *mockUserStore) GetUserByUsername(ctx context.Context, username string) (storage.User, error) {
	if m.getUserByUsernameFn == nil {
		panic("mockUserStore.GetUserByUsername: function not set")
	}
	return m.getUserByUsernameFn(ctx, username)
}

func chiCtx(req *http.Request, params ...string) *http.Request {
	rctx := chi.NewRouteContext()
	for i := 0; i < len(params); i += 2 {
		rctx.URLParams.Add(params[i], params[i+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func decodeBody(t testing.TB, body io.Reader, v any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(v); err != nil {
		t.Fatalf("decode body: %v", err)
	}
}

func makeToken(t testing.TB, secret []byte, username string) string {
	t.Helper()
	c := claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Username: username,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	s, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("makeToken: %v", err)
	}
	return s
}

type errResponse struct {
	Error string `json:"error"`
}
