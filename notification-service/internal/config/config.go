package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort           string
	GRPCPort          string
	KafkaBrokers      string
	KafkaTopicUserReg string
	KafkaGroupID      string

	// SMTP
	SMTPHost string
	SMTPPort string
	SMTPFrom string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppPort:           getEnv("APP_PORT", "8083"),
		GRPCPort:          getEnv("GRPC_PORT", "50056"),
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopicUserReg: getEnv("KAFKA_TOPIC_USER_REGISTERED", "user.registered"),
		KafkaGroupID:      getEnv("KAFKA_GROUP_ID", "notification-service"),

		SMTPHost: getEnv("SMTP_HOST", "mailhog"),
		SMTPPort: getEnv("SMTP_PORT", "1025"),
		SMTPFrom: getEnv("SMTP_FROM", "noreply@microservices.local"),
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
