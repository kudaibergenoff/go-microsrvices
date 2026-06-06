package main

import (
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
	"google.golang.org/grpc"

	"github.com/zhanuzak/microservices/product-service/internal/config"
	"github.com/zhanuzak/microservices/product-service/internal/handler"
	"github.com/zhanuzak/microservices/product-service/internal/repository"
	pb "github.com/zhanuzak/microservices/product-service/proto/product"
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

	productRepo := repository.NewProductRepository(db)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	grpcHandler := handler.NewGRPCProductHandler(productRepo)
	pb.RegisterProductServiceServer(grpcServer, grpcHandler)

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
		AppName: "Product Service",
	})

	app.Use(logger.New(logger.Config{
		Format:     "{\"time\":\"${time}\",\"status\":${status},\"latency\":\"${latency}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"path\":\"${path}\"}\n",
		TimeFormat: "2006-01-02T15:04:05Z07:00",
	}))
	app.Use(cors.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "product"})
	})

	productHandler := handler.NewProductHandler(productRepo)
	productHandler.SetupRoutes(app)

	go func() {
		log.Printf("HTTP server listening on :%s", cfg.AppPort)
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal("failed to start server: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down product service...")
	grpcServer.GracefulStop()
	_ = app.Shutdown()
	log.Println("product service stopped")
}

func migrate(db *sqlx.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS products (
		id BIGSERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		description TEXT DEFAULT '',
		price DECIMAL(10,2) NOT NULL,
		category VARCHAR(100) DEFAULT '',
		image_url TEXT DEFAULT '',
		stock INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}
