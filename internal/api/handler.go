package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"book-library/internal/logger"
	"book-library/internal/storage"
)

// BookStore is the interface for book storage operations.
// It is satisfied by the sqlc-generated *storage.Queries.
type BookStore interface {
	CreateBook(ctx context.Context, arg storage.CreateBookParams) (storage.Book, error)
	GetBook(ctx context.Context, id int32) (storage.Book, error)
	ListBooks(ctx context.Context) ([]storage.Book, error)
	UpdateBook(ctx context.Context, arg storage.UpdateBookParams) (storage.Book, error)
	DeleteBook(ctx context.Context, id int32) error
}

// Handler holds the dependencies for HTTP handlers.
type Handler struct {
	store     BookStore
	userStore UserStore
	jwtSecret []byte
}

// NewHandler creates a new Handler with the given store, userStore and JWT secret.
func NewHandler(store BookStore, userStore UserStore, jwtSecret []byte) *Handler {
	return &Handler{store: store, userStore: userStore, jwtSecret: jwtSecret}
}

// RegisterRoutes registers all routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", h.Register)
	mux.HandleFunc("POST /login", h.Login)

	mux.Handle("POST /books", h.AuthMiddleware(http.HandlerFunc(h.CreateBook)))
	mux.HandleFunc("GET /books/{id}", h.GetBook)
	mux.HandleFunc("GET /books", h.ListBooks)
	mux.HandleFunc("PUT /books/{id}", h.UpdateBook)
	mux.HandleFunc("DELETE /books/{id}", h.DeleteBook)
}

type createBookRequest struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int32  `json:"year"`
}

func (h *Handler) CreateBook(w http.ResponseWriter, r *http.Request) {
	var req createBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Debug("create book: invalid JSON", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	book, err := h.store.CreateBook(r.Context(), storage.CreateBookParams{
		Title:  req.Title,
		Author: req.Author,
		Year:   req.Year,
	})
	if err != nil {
		logger.Error("create book: db error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusCreated, book)
}

func (h *Handler) GetBook(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	book, err := h.store.GetBook(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Debug("get book: not found", "id", id)
		} else {
			logger.Error("get book: db error", err, "id", id)
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "book not found"})
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) ListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.ListBooks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list books"})
		return
	}

	if books == nil {
		books = []storage.Book{}
	}

	writeJSON(w, http.StatusOK, books)
}

type updateBookRequest struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Year   int32  `json:"year"`
}

func (h *Handler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req updateBookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	book, err := h.store.UpdateBook(r.Context(), storage.UpdateBookParams{
		ID:     id,
		Title:  req.Title,
		Author: req.Author,
		Year:   req.Year,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Debug("update book: not found", "id", id)
		} else {
			logger.Error("update book: db error", err, "id", id)
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "book not found"})
		return
	}

	writeJSON(w, http.StatusOK, book)
}

func (h *Handler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DeleteBook(r.Context(), id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			logger.Debug("delete book: not found", "id", id)
		} else {
			logger.Error("delete book: db error", err, "id", id)
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "book not found"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseID(r *http.Request) (int32, error) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(id), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
