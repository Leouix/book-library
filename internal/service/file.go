package service

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"

	"book-library/internal/storage/s3"
)

type FileService struct {
	storage s3.FileStorage
	baseURL string
}

func NewFileService(storage s3.FileStorage, baseURL string) *FileService {
	return &FileService{storage: storage, baseURL: baseURL}
}

func (s *FileService) UploadBookFile(ctx context.Context, originalName string, reader io.Reader, contentType string) (s3Key string, fileURL string, err error) {
	s3Key = fmt.Sprintf("books/%s/%s", uuid.New().String(), originalName)
	if err = s.storage.Upload(ctx, s3Key, reader, contentType); err != nil {
		return "", "", fmt.Errorf("s3 upload: %w", err)
	}
	fileURL = fmt.Sprintf("%s/%s", s.baseURL, s3Key)
	return
}
