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
}

func TestConfigLoad_CustomEnv(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("STORAGE_BASE_PATH", "/custom/storage")
	t.Setenv("DATABASE_URL", "postgres://user:pass@remote:5433/prod?sslmode=require")

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
}
