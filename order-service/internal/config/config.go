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

	KafkaBrokers string
	KafkaTopic   string

	ProductGRPCHost string
	ProductGRPCPort string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppPort:    getEnv("APP_PORT", "8086"),
		GRPCPort:   getEnv("GRPC_PORT", "50054"),
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "order_user"),
		DBPassword: getEnv("DB_PASSWORD", "order_password"),
		DBName:     getEnv("DB_NAME", "order_db"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopic:   getEnv("KAFKA_TOPIC_ORDERS", "order.events"),

		ProductGRPCHost: getEnv("PRODUCT_GRPC_HOST", "localhost"),
		ProductGRPCPort: getEnv("PRODUCT_GRPC_PORT", "50053"),
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

func (c *Config) ProductGRPCAddr() string {
	return c.ProductGRPCHost + ":" + c.ProductGRPCPort
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
