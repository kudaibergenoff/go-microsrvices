package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort      string
	GRPCPort     string
	DBHost       string
	DBPort       string
	DBUser       string
	DBPassword   string
	DBName       string
	DBSSLMode    string
	KafkaBrokers      string
	KafkaTopicUserReg string
	KafkaGroupID      string

	// Media Service gRPC
	MediaGRPCHost string
	MediaGRPCPort string

	// Keycloak
	KeycloakURL   string
	KeycloakRealm string

	// Tracing
	JaegerEndpoint string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppPort:      getEnv("APP_PORT", "8082"),
		GRPCPort:     getEnv("GRPC_PORT", "50052"),
		DBHost:       getEnv("DB_HOST", "localhost"),
		DBPort:       getEnv("DB_PORT", "5432"),
		DBUser:       getEnv("DB_USER", "user_user"),
		DBPassword:   getEnv("DB_PASSWORD", "user_password"),
		DBName:       getEnv("DB_NAME", "user_db"),
		DBSSLMode:    getEnv("DB_SSLMODE", "disable"),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopicUserReg: getEnv("KAFKA_TOPIC_USER_REGISTERED", "user.registered"),
		KafkaGroupID:      getEnv("KAFKA_GROUP_ID", "user-service"),

		MediaGRPCHost: getEnv("MEDIA_GRPC_HOST", "localhost"),
		MediaGRPCPort: getEnv("MEDIA_GRPC_PORT", "50055"),

		KeycloakURL:   getEnv("KEYCLOAK_URL", "http://localhost:8180"),
		KeycloakRealm: getEnv("KEYCLOAK_REALM", "microservices"),

		JaegerEndpoint: getEnv("JAEGER_ENDPOINT", "jaeger:4318"),
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

func (c *Config) MediaGRPCAddr() string {
	return c.MediaGRPCHost + ":" + c.MediaGRPCPort
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
