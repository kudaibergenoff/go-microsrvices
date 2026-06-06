package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"google.golang.org/grpc"

	"github.com/zhanuzak/microservices/media-service/internal/config"
	"github.com/zhanuzak/microservices/media-service/internal/handler"
	"github.com/zhanuzak/microservices/media-service/internal/storage"
	pb "github.com/zhanuzak/microservices/media-service/proto/media"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config: ", err)
	}

	minioStorage, err := storage.NewMinioStorage(
		cfg.MinioEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioBucket,
		cfg.MinioPublicURL,
		cfg.MinioUseSSL,
	)
	if err != nil {
		log.Fatal("failed to init minio storage: ", err)
	}
	log.Println("MinIO storage initialized")

	// Start gRPC server
	grpcServer := grpc.NewServer()
	grpcHandler := handler.NewGRPCMediaHandler(minioStorage)
	pb.RegisterMediaServiceServer(grpcServer, grpcHandler)

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
		AppName:   "Media Service",
		BodyLimit: 10 * 1024 * 1024, // 10 MB
	})

	app.Use(logger.New(logger.Config{
		Format:     "{\"time\":\"${time}\",\"status\":${status},\"latency\":\"${latency}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"path\":\"${path}\"}\n",
		TimeFormat: "2006-01-02T15:04:05Z07:00",
	}))
	app.Use(cors.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "media"})
	})

	mediaHandler := handler.NewMediaHandler(minioStorage)
	mediaHandler.SetupRoutes(app)

	go func() {
		log.Printf("HTTP server listening on :%s", cfg.AppPort)
		log.Printf("  MinIO → %s (bucket: %s)", cfg.MinioEndpoint, cfg.MinioBucket)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal("failed to start server: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down media service...")
	grpcServer.GracefulStop()
	_ = app.Shutdown()
	log.Println("media service stopped")
}
