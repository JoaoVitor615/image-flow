package config

import (
	"fmt"
	"os"
	"strings"
)

// Config encapsulates application configuration loaded from environment variables
type Config struct {
	Port            string
	DatabaseURL     string
	StorageBasePath string
	KafkaBrokers    []string
	KafkaTopic      string
}

// Load initializes configuration from environment variables with default values
func Load() *Config {
	brokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	brokers := strings.Split(brokersStr, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	return &Config{
		Port:            getEnv("PORT", "8080"),
		StorageBasePath: getEnv("STORAGE_BASE_PATH", "./data/images"),
		DatabaseURL:     buildDatabaseURL(),
		KafkaBrokers:    brokers,
		KafkaTopic:      getEnv("KAFKA_TOPIC", "image-uploaded"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func buildDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	pass := getEnv("DB_PASSWORD", "postgres")
	name := getEnv("DB_NAME", "imageflow")

	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, name)
}
