package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort  string
	GRPCPort string

	// MinIO
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool
	MinioPublicURL string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppPort:  getEnv("APP_PORT", "8084"),
		GRPCPort: getEnv("GRPC_PORT", "50055"),

		MinioEndpoint:  getEnv("MINIO_ENDPOINT", "minio:9000"),
		MinioAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:    getEnv("MINIO_BUCKET", "media"),
		MinioUseSSL:    false,
		MinioPublicURL: getEnv("MINIO_PUBLIC_URL", "http://localhost:9000"),
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
