package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"book-library/internal/logger"
)

const maxFileSize = 10 << 20 // 10 MB

func (h *Handler) UploadFile(w http.ResponseWriter, r *http.Request) {
	if h.fileSvc == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "file storage not configured"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxFileSize)

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file too large: max 10 MB"})
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

	f, err := h.fileSvc.UploadFile(r.Context(), header.Filename, file, contentType, header.Size)
	if err != nil {
		logger.Error("upload file", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to upload file"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": f.ID.String()})
}

func (h *Handler) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if h.fileSvc == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "file storage not configured"})
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid file id"})
		return
	}

	file, stream, err := h.fileSvc.DownloadFile(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "file not found"})
			return
		}
		logger.Error("download file", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to download file"})
		return
	}
	defer stream.Close()

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, file.OriginalName))
	io.Copy(w, stream)
}
