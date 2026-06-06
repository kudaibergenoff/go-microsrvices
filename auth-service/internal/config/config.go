package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort            string
	GRPCPort           string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	DBSSLMode          string
	KafkaBrokers       string
	KafkaTopicUserReg  string

	// Keycloak
	KeycloakURL          string
	KeycloakRealm        string
	KeycloakClientID     string
	KeycloakClientSecret string
	KeycloakAdminUser    string
	KeycloakAdminPassword string

	// Tracing
	JaegerEndpoint string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppPort:            getEnv("APP_PORT", "8081"),
		GRPCPort:           getEnv("GRPC_PORT", "50051"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "auth_user"),
		DBPassword:         getEnv("DB_PASSWORD", "auth_password"),
		DBName:             getEnv("DB_NAME", "auth_db"),
		DBSSLMode:          getEnv("DB_SSLMODE", "disable"),
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopicUserReg:  getEnv("KAFKA_TOPIC_USER_REGISTERED", "user.registered"),

		KeycloakURL:          getEnv("KEYCLOAK_URL", "http://localhost:8180"),
		KeycloakRealm:        getEnv("KEYCLOAK_REALM", "microservices"),
		KeycloakClientID:     getEnv("KEYCLOAK_CLIENT_ID", "auth-service"),
		KeycloakClientSecret: getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		KeycloakAdminUser:    getEnv("KEYCLOAK_ADMIN_USER", "admin"),
		KeycloakAdminPassword: getEnv("KEYCLOAK_ADMIN_PASSWORD", "admin"),

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

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
