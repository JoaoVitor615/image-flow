package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/JoaoVitor615/image-flow/internal/config"
	"github.com/JoaoVitor615/image-flow/internal/processor"
	"github.com/JoaoVitor615/image-flow/internal/queue"
	"github.com/JoaoVitor615/image-flow/internal/repository"
	"github.com/JoaoVitor615/image-flow/internal/storage"
	"github.com/JoaoVitor615/image-flow/internal/worker"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Set up signal trapping for graceful termination
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	// Initialize PostgreSQL connection pool
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Invalid database connection string: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Failed to create database connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Unable to reach database: %v", err)
	}
	log.Println("Connected to PostgreSQL successfully")

	// Initialize disk storage
	diskStorage, err := storage.NewDiskStorage(cfg.StorageBasePath)
	if err != nil {
		log.Fatalf("Failed to initialize disk storage at %s: %v", cfg.StorageBasePath, err)
	}
	log.Printf("Storage initialized at: %s", cfg.StorageBasePath)

	// Initialize Kafka consumer
	kafkaConsumer := queue.NewKafkaConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroupID)
	defer func() {
		if err := kafkaConsumer.Close(); err != nil {
			log.Printf("Error closing Kafka consumer: %v", err)
		}
	}()
	log.Printf("Kafka consumer initialized for topic: %s, group: %s", cfg.KafkaTopic, cfg.KafkaGroupID)

	// Initialize repository and image processing engine
	repo := repository.NewRepository(pool)
	imageProcessor := processor.NewImageProcessor()

	// Instantiate and start image worker with configured concurrency
	imageWorker := worker.NewImageWorker(
		kafkaConsumer,
		repo,
		diskStorage,
		imageProcessor,
		cfg.WorkerConcurrency,
	)

	if err := imageWorker.Start(ctx); err != nil {
		log.Fatalf("Worker terminated with error: %v", err)
	}
}
