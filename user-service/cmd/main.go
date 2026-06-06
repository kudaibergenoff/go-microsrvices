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
	"google.golang.org/grpc/credentials/insecure"

	"github.com/zhanuzak/microservices/user-service/internal/config"
	"github.com/zhanuzak/microservices/user-service/internal/handler"
	kafkaConsumer "github.com/zhanuzak/microservices/user-service/internal/kafka"
	"github.com/zhanuzak/microservices/user-service/internal/metrics"
	"github.com/zhanuzak/microservices/user-service/internal/repository"
	"github.com/zhanuzak/microservices/user-service/internal/service"
	"github.com/zhanuzak/microservices/user-service/internal/tracing"
	mediapb "github.com/zhanuzak/microservices/user-service/proto/media"
	userpb "github.com/zhanuzak/microservices/user-service/proto/user"
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

	// Initialize tracing
	shutdownTracer, err := tracing.Init(context.Background(), "user-service", cfg.JaegerEndpoint)
	if err != nil {
		log.Printf("WARNING: failed to init tracer: %v", err)
	} else {
		defer shutdownTracer(context.Background())
	}

	// gRPC connection to Media Service
	mediaConn, err := grpc.NewClient(cfg.MediaGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatal("failed to connect to media service: ", err)
	}
	defer mediaConn.Close()
	mediaClient := mediapb.NewMediaServiceClient(mediaConn)

	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo, mediaClient)

	// Start Kafka consumer with cancellable context
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	consumer := kafkaConsumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopicUserReg, cfg.KafkaGroupID, userRepo)
	defer consumer.Close()
	go consumer.Start(consumerCtx)

	// Start gRPC server (with trace propagation)
	grpcServer := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	grpcHandler := handler.NewGRPCUserHandler(userService)
	userpb.RegisterUserServiceServer(grpcServer, grpcHandler)

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
		AppName: "User Service",
	})

	app.Use(logger.New(logger.Config{
		Format:     "{\"time\":\"${time}\",\"status\":${status},\"latency\":\"${latency}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"path\":\"${path}\"}\n",
		TimeFormat: "2006-01-02T15:04:05Z07:00",
	}))
	app.Use(cors.New())
	app.Use(metrics.Middleware())
	app.Use(tracing.Middleware("user-service"))

	app.Get("/metrics", metrics.Handler())
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "user"})
	})

	authMiddleware := handler.KeycloakAuthMiddleware(cfg.KeycloakIssuerURL())
	userHandler := handler.NewUserHandler(userService)
	userHandler.SetupRoutes(app, authMiddleware)

	go func() {
		log.Printf("HTTP server listening on :%s", cfg.AppPort)
		log.Printf("  Keycloak → %s", cfg.KeycloakIssuerURL())
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal("failed to start server: ", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down user service...")
	consumerCancel()
	grpcServer.GracefulStop()
	_ = app.Shutdown()
	log.Println("user service stopped")
}

func migrate(db *sqlx.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGSERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		name VARCHAR(255) NOT NULL,
		avatar_url TEXT NOT NULL DEFAULT '',
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Add avatar_url column if it doesn't exist (for existing tables)
	_, _ = db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT NOT NULL DEFAULT ''`)

	return nil
}
