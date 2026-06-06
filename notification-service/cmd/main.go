package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc"

	"github.com/zhanuzak/microservices/notification-service/internal/config"
	"github.com/zhanuzak/microservices/notification-service/internal/handler"
	kafkaConsumer "github.com/zhanuzak/microservices/notification-service/internal/kafka"
	"github.com/zhanuzak/microservices/notification-service/internal/smtp"
	pb "github.com/zhanuzak/microservices/notification-service/proto/notification"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config: ", err)
	}

	mailer := smtp.NewMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFrom)

	// Start Kafka consumer
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	consumer := kafkaConsumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopicUserReg, cfg.KafkaGroupID, mailer)
	defer consumer.Close()
	go consumer.Start(consumerCtx)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	grpcHandler := handler.NewGRPCNotificationHandler(mailer)
	pb.RegisterNotificationServiceServer(grpcServer, grpcHandler)

	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			log.Fatal("failed to listen grpc: ", err)
		}
		log.Printf("gRPC server listening on :%s", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("failed to serve grpc: ", err)
		}
	}()

	// HTTP server for health check
	app := fiber.New(fiber.Config{
		AppName: "Notification Service",
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "notification"})
	})

	go func() {
		log.Printf("HTTP server listening on :%s", cfg.AppPort)
		log.Printf("  Kafka → %s (topic: %s)", cfg.KafkaBrokers, cfg.KafkaTopicUserReg)
		log.Printf("  SMTP  → %s:%s", cfg.SMTPHost, cfg.SMTPPort)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal("failed to start server: ", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down notification service...")
	consumerCancel()
	grpcServer.GracefulStop()
	_ = app.Shutdown()
	log.Println("notification service stopped")
}
