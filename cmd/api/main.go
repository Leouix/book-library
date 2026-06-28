package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"book-library/internal/api"
	"book-library/internal/logger"
	"book-library/internal/service"
	"book-library/internal/storage"
	s3storage "book-library/internal/storage/s3"
)

func runMigrations(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migrate up: %w", err)
	}

	return nil
}

func main() {
	if err := godotenv.Load(); err != nil {
		logger.Warn(".env file not found, using environment variables")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://bookuser:bookpass@localhost:5432/bookdb"
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "changeme"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		logger.Fatal("failed to connect to database", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("failed to ping database", err)
	}
	logger.Info("connected to database")

	if err := runMigrations(dsn); err != nil {
		logger.Fatal("failed to run migrations", err)
	}

	queries := storage.New(pool)

	b2KeyID := os.Getenv("B2_KEY_ID")
	b2AppKey := os.Getenv("B2_APPLICATION_KEY")
	b2Region := os.Getenv("B2_REGION")
	b2Endpoint := os.Getenv("B2_ENDPOINT")
	b2Bucket := os.Getenv("B2_BUCKET")

	fileBaseURL := os.Getenv("FILE_BASE_URL")
	if fileBaseURL == "" {
		fileBaseURL = "http://localhost:8080"
	}

	var fileSvc *service.FileService
	if b2KeyID != "" && b2AppKey != "" && b2Endpoint != "" && b2Bucket != "" {
		s3Client, err := s3storage.NewClient(ctx, b2KeyID, b2AppKey, b2Region, b2Endpoint)
		if err != nil {
			logger.Fatal("failed to create S3 client", err)
		}
		fileStorage := s3storage.NewStorage(s3Client, b2Bucket)
		fileSvc = service.NewFileService(fileStorage, fileBaseURL)
		logger.Info("S3 file storage initialized")
	} else {
		logger.Warn("B2 credentials not set, file upload/download disabled")
	}

	workerPool := service.NewWorkerPool(3, queries, fileSvc)

	pendingBooks, err := queries.GetPendingBooks(ctx)
	if err == nil {
		for _, b := range pendingBooks {
			logger.Warn("marking stale book as failed", "id", b.ID, "status", b.Status)
			queries.FailBook(ctx, b.ID)
		}
	}
	if len(pendingBooks) > 0 {
		logger.Info("stale books marked as failed", "count", len(pendingBooks))
	}

	workerPool.Start()

	handler := api.NewHandler(queries, queries, fileSvc, workerPool, []byte(jwtSecret))

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	handler.RegisterRoutes(r)

	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		logger.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Fatal("server forced to shutdown", err)
	}

	workerCtx, workerCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer workerCancel()

	if err := workerPool.Shutdown(workerCtx); err != nil {
		logger.Warn("worker pool shutdown timed out")
	}

	logger.Info("server stopped")
}
