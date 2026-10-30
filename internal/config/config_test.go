package config_test

import (
	"os"
	"testing"

	"github.com/JoaoVitor615/image-flow/internal/config"
)

func TestConfigLoad_Defaults(t *testing.T) {
	// Clear relevant env vars
	os.Unsetenv("PORT")
	os.Unsetenv("STORAGE_BASE_PATH")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("KAFKA_BROKERS")
	os.Unsetenv("KAFKA_TOPIC")
	os.Unsetenv("KAFKA_GROUP_ID")
	os.Unsetenv("WORKER_CONCURRENCY")

	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Fatalf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.StorageBasePath != "./data/images" {
		t.Fatalf("expected default storage path ./data/images, got %s", cfg.StorageBasePath)
	}
	expectedDB := "postgres://postgres:postgres@localhost:5432/imageflow?sslmode=disable"
	if cfg.DatabaseURL != expectedDB {
		t.Fatalf("expected default db url %s, got %s", expectedDB, cfg.DatabaseURL)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Fatalf("expected default kafka broker localhost:9092, got %v", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "image-uploaded" {
		t.Fatalf("expected default kafka topic image-uploaded, got %s", cfg.KafkaTopic)
	}
	if cfg.KafkaGroupID != "image-processing-group" {
		t.Fatalf("expected default kafka group id image-processing-group, got %s", cfg.KafkaGroupID)
	}
	if cfg.WorkerConcurrency != 5 {
		t.Fatalf("expected default worker concurrency 5, got %d", cfg.WorkerConcurrency)
	}
}

func TestConfigLoad_CustomEnv(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("STORAGE_BASE_PATH", "/custom/storage")
	t.Setenv("DATABASE_URL", "postgres://user:pass@remote:5433/prod?sslmode=require")
	t.Setenv("KAFKA_BROKERS", "broker1:9092, broker2:9092")
	t.Setenv("KAFKA_TOPIC", "custom-topic")
	t.Setenv("KAFKA_GROUP_ID", "custom-group")
	t.Setenv("WORKER_CONCURRENCY", "10")

	cfg := config.Load()

	if cfg.Port != "9000" {
		t.Fatalf("expected custom port 9000, got %s", cfg.Port)
	}
	if cfg.StorageBasePath != "/custom/storage" {
		t.Fatalf("expected custom storage path /custom/storage, got %s", cfg.StorageBasePath)
	}
	if cfg.DatabaseURL != "postgres://user:pass@remote:5433/prod?sslmode=require" {
		t.Fatalf("unexpected custom db url: %s", cfg.DatabaseURL)
	}
	if len(cfg.KafkaBrokers) != 2 || cfg.KafkaBrokers[0] != "broker1:9092" || cfg.KafkaBrokers[1] != "broker2:9092" {
		t.Fatalf("expected custom kafka brokers [broker1:9092, broker2:9092], got %v", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "custom-topic" {
		t.Fatalf("expected custom kafka topic custom-topic, got %s", cfg.KafkaTopic)
	}
	if cfg.KafkaGroupID != "custom-group" {
		t.Fatalf("expected custom kafka group id custom-group, got %s", cfg.KafkaGroupID)
	}
	if cfg.WorkerConcurrency != 10 {
		t.Fatalf("expected custom worker concurrency 10, got %d", cfg.WorkerConcurrency)
	}
}
