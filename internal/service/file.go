package service

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"book-library/internal/storage"
	"book-library/internal/storage/s3"
)

type FileService struct {
	storage s3.FileStorage
	queries *storage.Queries
}

func NewFileService(storage s3.FileStorage, queries *storage.Queries) *FileService {
	return &FileService{storage: storage, queries: queries}
}

func (s *FileService) UploadFile(ctx context.Context, originalName string, reader io.Reader, contentType string, size int64) (*storage.File, error) {
	s3Key := uuid.New().String()

	if err := s.storage.Upload(ctx, s3Key, reader, contentType); err != nil {
		return nil, fmt.Errorf("s3 upload: %w", err)
	}

	file, err := s.queries.CreateFile(ctx, storage.CreateFileParams{
		OriginalName: originalName,
		S3Key:        s3Key,
		MimeType:     contentType,
		Size:         size,
	})
	if err != nil {
		return nil, fmt.Errorf("db create file: %w", err)
	}

	return &file, nil
}

func (s *FileService) DownloadFile(ctx context.Context, id uuid.UUID) (*storage.File, io.ReadCloser, error) {
	var dbID pgtype.UUID
	dbID.Bytes = id
	dbID.Valid = true

	file, err := s.queries.GetFile(ctx, dbID)
	if err != nil {
		return nil, nil, fmt.Errorf("db get file: %w", err)
	}

	reader, err := s.storage.Download(ctx, file.S3Key)
	if err != nil {
		return nil, nil, fmt.Errorf("s3 download: %w", err)
	}

	return &file, reader, nil
}
