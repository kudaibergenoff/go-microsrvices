package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"github.com/zhanuzak/microservices/auth-service/internal/config"
	"github.com/zhanuzak/microservices/auth-service/internal/handler"
	"github.com/zhanuzak/microservices/auth-service/internal/metrics"
	"github.com/zhanuzak/microservices/auth-service/internal/tracing"
	kafkaProducer "github.com/zhanuzak/microservices/auth-service/internal/kafka"
	"github.com/zhanuzak/microservices/auth-service/internal/keycloak"
	"github.com/zhanuzak/microservices/auth-service/internal/repository"
	"github.com/zhanuzak/microservices/auth-service/internal/service"
	pb "github.com/zhanuzak/microservices/auth-service/proto/auth"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config: ", err)
	}

	db, err := sqlx.Connect("postgres", cfg.DSN())
	if err != nil {
		log.Fatal("failed to connect to database: ", err)
	}
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatal("failed to run migrations: ", err)
	}

	log.Println("database connected and migrated")

	producer := kafkaProducer.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopicUserReg)
	defer producer.Close()

	kcClient := keycloak.NewClient(
		cfg.KeycloakURL,
		cfg.KeycloakRealm,
		cfg.KeycloakClientID,
		cfg.KeycloakClientSecret,
		cfg.KeycloakAdminUser,
		cfg.KeycloakAdminPassword,
	)

	if err := kcClient.InitVerifier(context.Background()); err != nil {
		log.Printf("WARNING: OIDC verifier init failed (token validation will be unavailable): %v", err)
	} else {
		log.Println("OIDC verifier initialized successfully")
	}

	// Initialize tracing
	shutdownTracer, err := tracing.Init(context.Background(), "auth-service", cfg.JaegerEndpoint)
	if err != nil {
		log.Printf("WARNING: failed to init tracer: %v", err)
	} else {
		defer shutdownTracer(context.Background())
	}

	userRepo := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepo, producer, kcClient)

	// Start gRPC server (with trace propagation)
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	grpcHandler := handler.NewGRPCAuthHandler(authService)
	pb.RegisterAuthServiceServer(grpcServer, grpcHandler)

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

	// Start Fiber HTTP server
	app := fiber.New(fiber.Config{
		AppName: "Auth Service",
	})

	app.Use(logger.New(logger.Config{
		Format:     "{\"time\":\"${time}\",\"status\":${status},\"latency\":\"${latency}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"path\":\"${path}\"}\n",
		TimeFormat: "2006-01-02T15:04:05Z07:00",
	}))
	app.Use(cors.New())
	app.Use(metrics.Middleware())
	app.Use(tracing.Middleware("auth-service"))

	app.Get("/metrics", metrics.Handler())
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "auth"})
	})

	authHandler := handler.NewAuthHandler(authService)
	authHandler.SetupRoutes(app)

	go func() {
		log.Printf("HTTP server listening on :%s", cfg.AppPort)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal("failed to start server: ", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down auth service...")
	grpcServer.GracefulStop()
	_ = app.Shutdown()
	log.Println("auth service stopped")
}

func migrate(db *sqlx.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}
