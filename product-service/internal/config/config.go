package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort    string
	GRPCPort   string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppPort:    getEnv("APP_PORT", "8085"),
		GRPCPort:   getEnv("GRPC_PORT", "50053"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "product_user"),
		DBPassword: getEnv("DB_PASSWORD", "product_password"),
		DBName:     getEnv("DB_NAME", "product_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),
	}, nil
}

func (c *Config) DSN() string {
	return "host=" + c.DBHost +
		" port=" + c.DBPort +
		" user=" + c.DBUser +
		" password=" + c.DBPassword +
		" dbname=" + c.DBName +
		" sslmode=" + c.DBSSLMode
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
