package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"book-library/internal/logger"
	"book-library/internal/service"
	"book-library/internal/storage"
)

const maxFileSize = 10 << 20 // 10 MB

type BookStore interface {
	CreatePendingBook(ctx context.Context, arg storage.CreatePendingBookParams) (storage.Book, error)
	GetBook(ctx context.Context, id int32) (storage.Book, error)
	ListBooks(ctx context.Context) ([]storage.Book, error)
	UpdateBook(ctx context.Context, arg storage.UpdateBookParams) (storage.Book, error)
	DeleteBook(ctx context.Context, id int32) error
}

type Handler struct {
	store      BookStore
	userStore  UserStore
	fileSvc    *service.FileService
	workerPool *service.WorkerPool
	jwtSecret  []byte
}

func NewHandler(store BookStore, userStore UserStore, fileSvc *service.FileService, workerPool *service.WorkerPool, jwtSecret []byte) *Handler {
	return &Handler{store: store, userStore: userStore, fileSvc: fileSvc, workerPool: workerPool, jwtSecret: jwtSecret}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/register", h.Register)
	r.Post("/login", h.Login)

	r.With(h.AuthMiddleware).Post("/books", h.CreateBook)
	r.Get("/books", h.ListBooks)
	r.Get("/books/{id}", h.GetBook)
	r.With(h.AuthMiddleware).Put("/books/{id}", h.UpdateBook)
	r.With(h.AuthMiddleware).Delete("/books/{id}", h.DeleteBook)
}

func (h *Handler) CreateBook(w http.ResponseWriter, r *http.Request) {
	if h.fileSvc == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "file storage not configured"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file too large: max 10 MB"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	title := r.FormValue("title")
	author := r.FormValue("author")
	yearStr := r.FormValue("year")

	if title == "" || author == "" || yearStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title, author and year are required"})
		return
	}

	year, err := strconv.ParseInt(yearStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "year must be a valid integer"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file field"})
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}

	tmpFile, err := os.CreateTemp("", "book-*")
	if err != nil {
		logger.Error("create book: create temp file", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
		return
	}
	tmpPath := tmpFile.Name()

	_, err = io.Copy(tmpFile, file)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		logger.Error("create book: copy to temp file", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
		return
	}

	book, err := h.store.CreatePendingBook(r.Context(), storage.CreatePendingBookParams{
		Title:  title,
		Author: author,
		Year:   int32(year),
	})
	if err != nil {
		os.Remove(tmpPath)
		logger.Error("create book: db error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	h.workerPool.Enqueue(service.Job{
		BookID:   book.ID,
		FilePath: tmpPath,
		FileName: header.Filename,
		MimeType: contentType,
	})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"id":     book.ID,
		"status": "pending",
	})
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

	if book.Status != "completed" {
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

func (h *Handler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req struct {
		Title  string `json:"title"`
		Author string `json:"author"`
		Year   int32  `json:"year"`
	}
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
	idStr := chi.URLParam(r, "id")
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
