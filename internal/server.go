package internal

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/JoaoVitor615/image-flow/internal/config"
	"github.com/JoaoVitor615/image-flow/internal/controller"
	"github.com/JoaoVitor615/image-flow/internal/repository"
	"github.com/JoaoVitor615/image-flow/internal/service"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server encapsulates dependencies and HTTP lifecycle
type Server struct {
	httpServer *http.Server
	pool       *pgxpool.Pool
	port       string
}

// NewServer initializes dependencies using the provided configuration
func NewServer(ctx context.Context, cfg *config.Config) (*Server, error) {
	// Initialize PostgreSQL pool
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to reach database: %w", err)
	}
	log.Println("Connected to PostgreSQL successfully")

	// Initialize disk storage
	diskStorage, err := storage.NewDiskStorage(cfg.StorageBasePath)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to initialize disk storage at %s: %w", cfg.StorageBasePath, err)
	}
	log.Printf("Storage initialized at: %s", cfg.StorageBasePath)

	// Assemble layered architecture
	repo := repository.NewRepository(pool)
	imageService := service.NewImageService(diskStorage, repo)
	imageController := controller.NewImageController(imageService)

	// Router and middleware stack
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	router.Post("/images", imageController.Upload)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	return &Server{
		httpServer: httpServer,
		pool:       pool,
		port:       cfg.Port,
	}, nil
}

// Run starts the HTTP server and handles graceful shutdown when context is cancelled
func (s *Server) Run(ctx context.Context) error {
	serverErr := make(chan error, 1)

	go func() {
		log.Printf("API server running on port %s", s.port)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	select {
	case err := <-serverErr:
		s.pool.Close()
		return err
	case <-ctx.Done():
		log.Println("Shutting down API server gracefully...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	defer s.pool.Close()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("failed to shutdown HTTP server: %w", err)
	}

	log.Println("API server stopped")
	return nil
}
