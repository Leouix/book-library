package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5"

	"book-library/internal/storage"
)

func TestListBooks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mock       *mockBookStore
		wantStatus int
		wantBooks  int
	}{
		{
			name: "empty list",
			mock: &mockBookStore{
				listBooksFn: func(_ context.Context) ([]storage.Book, error) {
					return nil, nil
				},
			},
			wantStatus: http.StatusOK,
			wantBooks:  0,
		},
		{
			name: "with books",
			mock: &mockBookStore{
				listBooksFn: func(_ context.Context) ([]storage.Book, error) {
					return []storage.Book{{ID: 1, Title: "T", Author: "A", Year: 2024}}, nil
				},
			},
			wantStatus: http.StatusOK,
			wantBooks:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.mock, &mockUserStore{}, nil, []byte("secret"))
			req := httptest.NewRequest(http.MethodGet, "/books", nil)
			rec := httptest.NewRecorder()

			h.ListBooks(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var books []storage.Book
			decodeBody(t, rec.Body, &books)
			if len(books) != tt.wantBooks {
				t.Errorf("got %d books, want %d", len(books), tt.wantBooks)
			}
		})
	}
}

func TestListBooks_dbError(t *testing.T) {
	t.Parallel()

	h := NewHandler(&mockBookStore{
		listBooksFn: func(_ context.Context) ([]storage.Book, error) {
			return nil, pgx.ErrNoRows
		},
	}, &mockUserStore{}, nil, []byte("secret"))
	req := httptest.NewRequest(http.MethodGet, "/books", nil)
	rec := httptest.NewRecorder()

	h.ListBooks(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	var e errResponse
	decodeBody(t, rec.Body, &e)
	if e.Error == "" {
		t.Error("expected error message")
	}
}

func TestGetBook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		mock       *mockBookStore
		wantStatus int
		wantBook   *storage.Book
	}{
		{
			name: "success",
			id:   "1",
			mock: &mockBookStore{
				getBookFn: func(_ context.Context, id int32) (storage.Book, error) {
					return storage.Book{ID: id, Title: "T", Author: "A", Year: 2024}, nil
				},
			},
			wantStatus: http.StatusOK,
			wantBook:   &storage.Book{ID: 1, Title: "T", Author: "A", Year: 2024},
		},
		{
			name: "not found",
			id:   "999",
			mock: &mockBookStore{
				getBookFn: func(_ context.Context, id int32) (storage.Book, error) {
					return storage.Book{}, pgx.ErrNoRows
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			id:         "abc",
			mock:       &mockBookStore{getBookFn: func(_ context.Context, _ int32) (storage.Book, error) { panic("should not be called") }},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.mock, &mockUserStore{}, nil, []byte("secret"))
			req := chiCtx(httptest.NewRequest(http.MethodGet, "/books/"+tt.id, nil), "id", tt.id)
			rec := httptest.NewRecorder()

			h.GetBook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantBook != nil {
				var got storage.Book
				decodeBody(t, rec.Body, &got)
				if got != *tt.wantBook {
					t.Errorf("got %+v, want %+v", got, *tt.wantBook)
				}
			}
		})
	}
}

func TestCreateBook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       any
		mock       *mockBookStore
		wantStatus int
	}{
		{
			name: "success",
			body: map[string]any{"title": "T", "author": "A", "year": 2024},
			mock: &mockBookStore{
				createBookFn: func(_ context.Context, arg storage.CreateBookParams) (storage.Book, error) {
					return storage.Book{ID: 1, Title: arg.Title, Author: arg.Author, Year: arg.Year}, nil
				},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid JSON",
			body:       "<<<>>>",
			mock:       &mockBookStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "db error",
			body: map[string]any{"title": "T", "author": "A", "year": 2024},
			mock: &mockBookStore{
				createBookFn: func(_ context.Context, _ storage.CreateBookParams) (storage.Book, error) {
					return storage.Book{}, pgx.ErrNoRows
				},
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(tt.body); err != nil {
				t.Fatal(err)
			}

			h := NewHandler(tt.mock, &mockUserStore{}, nil, []byte("secret"))
			req := httptest.NewRequest(http.MethodPost, "/books", &buf)
			rec := httptest.NewRecorder()

			h.CreateBook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestUpdateBook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		body       any
		mock       *mockBookStore
		wantStatus int
	}{
		{
			name: "success",
			id:   "1",
			body: map[string]any{"title": "T", "author": "A", "year": 2024},
			mock: &mockBookStore{
				updateBookFn: func(_ context.Context, arg storage.UpdateBookParams) (storage.Book, error) {
					return storage.Book{ID: arg.ID, Title: arg.Title, Author: arg.Author, Year: arg.Year}, nil
				},
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "not found",
			id:   "999",
			body: map[string]any{"title": "T", "author": "A", "year": 2024},
			mock: &mockBookStore{
				updateBookFn: func(_ context.Context, _ storage.UpdateBookParams) (storage.Book, error) {
					return storage.Book{}, pgx.ErrNoRows
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			id:         "abc",
			body:       map[string]any{"title": "T", "author": "A", "year": 2024},
			mock:       &mockBookStore{},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid JSON",
			id:         "1",
			body:       "<<<>>>",
			mock:       &mockBookStore{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(tt.body); err != nil {
				t.Fatal(err)
			}

			h := NewHandler(tt.mock, &mockUserStore{}, nil, []byte("secret"))
			req := chiCtx(httptest.NewRequest(http.MethodPut, "/books/"+tt.id, &buf), "id", tt.id)
			rec := httptest.NewRecorder()

			h.UpdateBook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestDeleteBook(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		mock       *mockBookStore
		wantStatus int
	}{
		{
			name: "success",
			id:   "1",
			mock: &mockBookStore{
				deleteBookFn: func(_ context.Context, _ int32) error {
					return nil
				},
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "not found",
			id:   "999",
			mock: &mockBookStore{
				deleteBookFn: func(_ context.Context, _ int32) error {
					return pgx.ErrNoRows
				},
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid id",
			id:         "abc",
			mock:       &mockBookStore{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewHandler(tt.mock, &mockUserStore{}, nil, []byte("secret"))
			req := chiCtx(httptest.NewRequest(http.MethodDelete, "/books/"+tt.id, nil), "id", tt.id)
			rec := httptest.NewRecorder()

			h.DeleteBook(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
