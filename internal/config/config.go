package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config encapsulates application configuration loaded from environment variables
type Config struct {
	Port              string
	DatabaseURL       string
	StorageBasePath   string
	KafkaBrokers      []string
	KafkaTopic        string
	KafkaGroupID      string
	WorkerConcurrency int
}

// Load initializes configuration from environment variables with default values
func Load() *Config {
	brokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	brokers := strings.Split(brokersStr, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	concurrency := 5
	if cStr := os.Getenv("WORKER_CONCURRENCY"); cStr != "" {
		if cVal, err := strconv.Atoi(cStr); err == nil && cVal > 0 {
			concurrency = cVal
		}
	}

	return &Config{
		Port:              getEnv("PORT", "8080"),
		StorageBasePath:   getEnv("STORAGE_BASE_PATH", "./data/images"),
		DatabaseURL:       buildDatabaseURL(),
		KafkaBrokers:      brokers,
		KafkaTopic:        getEnv("KAFKA_TOPIC", "image-uploaded"),
		KafkaGroupID:      getEnv("KAFKA_GROUP_ID", "image-processing-group"),
		WorkerConcurrency: concurrency,
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
