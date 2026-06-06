package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort      string
	AuthGRPCHost string
	AuthGRPCPort string
	UserGRPCHost    string
	UserGRPCPort    string
	ProductGRPCHost string
	ProductGRPCPort string
	OrderGRPCHost        string
	OrderGRPCPort        string
	MediaGRPCHost        string
	MediaGRPCPort        string
	NotificationGRPCHost string
	NotificationGRPCPort string

	// Keycloak
	KeycloakURL   string
	KeycloakRealm string

	// Tracing
	JaegerEndpoint string

	// Rate Limiting
	RateLimitMax    int
	RateLimitWindow int // seconds
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppPort:      getEnv("APP_PORT", "8080"),
		AuthGRPCHost: getEnv("AUTH_GRPC_HOST", "localhost"),
		AuthGRPCPort: getEnv("AUTH_GRPC_PORT", "50051"),
		UserGRPCHost:    getEnv("USER_GRPC_HOST", "localhost"),
		UserGRPCPort:    getEnv("USER_GRPC_PORT", "50052"),
		ProductGRPCHost: getEnv("PRODUCT_GRPC_HOST", "localhost"),
		ProductGRPCPort: getEnv("PRODUCT_GRPC_PORT", "50053"),
		OrderGRPCHost:        getEnv("ORDER_GRPC_HOST", "localhost"),
		OrderGRPCPort:        getEnv("ORDER_GRPC_PORT", "50054"),
		MediaGRPCHost:        getEnv("MEDIA_GRPC_HOST", "localhost"),
		MediaGRPCPort:        getEnv("MEDIA_GRPC_PORT", "50055"),
		NotificationGRPCHost: getEnv("NOTIFICATION_GRPC_HOST", "localhost"),
		NotificationGRPCPort: getEnv("NOTIFICATION_GRPC_PORT", "50056"),

		KeycloakURL:   getEnv("KEYCLOAK_URL", "http://localhost:8180"),
		KeycloakRealm: getEnv("KEYCLOAK_REALM", "microservices"),

		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "jaeger:4318"),

		RateLimitMax:    getEnvInt("RATE_LIMIT_MAX", 100),
		RateLimitWindow: getEnvInt("RATE_LIMIT_WINDOW", 60),
	}, nil
}

func (c *Config) AuthGRPCAddr() string {
	return c.AuthGRPCHost + ":" + c.AuthGRPCPort
}

func (c *Config) UserGRPCAddr() string {
	return c.UserGRPCHost + ":" + c.UserGRPCPort
}

func (c *Config) ProductGRPCAddr() string {
	return c.ProductGRPCHost + ":" + c.ProductGRPCPort
}

func (c *Config) OrderGRPCAddr() string {
	return c.OrderGRPCHost + ":" + c.OrderGRPCPort
}

func (c *Config) MediaGRPCAddr() string {
	return c.MediaGRPCHost + ":" + c.MediaGRPCPort
}

func (c *Config) NotificationGRPCAddr() string {
	return c.NotificationGRPCHost + ":" + c.NotificationGRPCPort
}

func (c *Config) KeycloakIssuerURL() string {
	return c.KeycloakURL + "/realms/" + c.KeycloakRealm
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
