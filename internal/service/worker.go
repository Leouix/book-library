package service

import (
	"context"
	"io"
	"os"
	"sync"

	"book-library/internal/logger"
	"book-library/internal/storage"
)

type Job struct {
	BookID   int32
	FilePath string
	FileName string
	MimeType string
}

type BookProcessor interface {
	CompleteBook(ctx context.Context, arg storage.CompleteBookParams) error
	FailBook(ctx context.Context, id int32) error
}

type WorkerPool struct {
	jobs    chan Job
	store   BookProcessor
	fileSvc *FileService
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	workers int
}

func NewWorkerPool(numWorkers int, store BookProcessor, fileSvc *FileService) *WorkerPool {
	_, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		jobs:    make(chan Job, 100),
		store:   store,
		fileSvc: fileSvc,
		cancel:  cancel,
		workers: numWorkers,
	}
}

func (wp *WorkerPool) Start() {
	for i := range wp.workers {
		wp.wg.Add(1)
		go wp.worker(i + 1)
	}
}

func (wp *WorkerPool) Enqueue(job Job) {
	wp.jobs <- job
}

func (wp *WorkerPool) Shutdown(ctx context.Context) error {
	close(wp.jobs)

	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	logger.Info("worker started", "worker_id", id)

	for job := range wp.jobs {
		logger.Info("worker processing job", "worker_id", id, "book_id", job.BookID)
		wp.process(job)
	}

	logger.Info("worker stopped", "worker_id", id)
}

func (wp *WorkerPool) process(job Job) {
	f, err := os.Open(job.FilePath)
	if err != nil {
		logger.Error("worker: open temp file", err, "book_id", job.BookID, "path", job.FilePath)
		wp.store.FailBook(context.Background(), job.BookID)
		os.Remove(job.FilePath)
		return
	}
	defer f.Close()

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		logger.Error("worker: seek temp file", err, "book_id", job.BookID)
		wp.store.FailBook(context.Background(), job.BookID)
		f.Close()
		os.Remove(job.FilePath)
		return
	}

	s3Key, fileURL, err := wp.fileSvc.UploadBookFile(context.Background(), job.FileName, f, job.MimeType)
	f.Close()
	if err != nil {
		logger.Error("worker: upload to s3", err, "book_id", job.BookID)
		wp.store.FailBook(context.Background(), job.BookID)
		os.Remove(job.FilePath)
		return
	}

	err = wp.store.CompleteBook(context.Background(), storage.CompleteBookParams{
		ID:       job.BookID,
		FileUrl:  fileURL,
		S3Key:    s3Key,
		FileName: job.FileName,
	})
	if err != nil {
		logger.Error("worker: complete book in db", err, "book_id", job.BookID)
		os.Remove(job.FilePath)
		return
	}

	os.Remove(job.FilePath)
	logger.Info("worker: book processed", "book_id", job.BookID, "s3_key", s3Key)
}
