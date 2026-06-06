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
	"google.golang.org/grpc/credentials/insecure"

	"github.com/zhanuzak/microservices/order-service/internal/config"
	"github.com/zhanuzak/microservices/order-service/internal/handler"
	kafkaProducer "github.com/zhanuzak/microservices/order-service/internal/kafka"
	"github.com/zhanuzak/microservices/order-service/internal/repository"
	orderpb "github.com/zhanuzak/microservices/order-service/proto/order"
	productpb "github.com/zhanuzak/microservices/order-service/proto/product"
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

	producer := kafkaProducer.NewProducer(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer producer.Close()

	// gRPC connection to Product Service
	productConn, err := grpc.NewClient(cfg.ProductGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal("failed to connect to product service: ", err)
	}
	defer productConn.Close()
	productClient := productpb.NewProductServiceClient(productConn)

	orderRepo := repository.NewOrderRepository(db)

	// Start gRPC server
	grpcServer := grpc.NewServer()
	grpcHandler := handler.NewGRPCOrderHandler(orderRepo, producer, productClient)
	orderpb.RegisterOrderServiceServer(grpcServer, grpcHandler)

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
		AppName: "Order Service",
	})

	app.Use(logger.New(logger.Config{
		Format:     "{\"time\":\"${time}\",\"status\":${status},\"latency\":\"${latency}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"path\":\"${path}\"}\n",
		TimeFormat: "2006-01-02T15:04:05Z07:00",
	}))
	app.Use(cors.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "order"})
	})

	orderHandler := handler.NewOrderHandler(orderRepo, producer)
	orderHandler.SetupRoutes(app)

	go func() {
		log.Printf("HTTP server listening on :%s", cfg.AppPort)
		log.Printf("  Product gRPC → %s", cfg.ProductGRPCAddr())
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal("failed to start server: ", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down order service...")
	grpcServer.GracefulStop()
	_ = app.Shutdown()
	log.Println("order service stopped")
}

func migrate(db *sqlx.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS orders (
		id BIGSERIAL PRIMARY KEY,
		user_id BIGINT NOT NULL,
		product_id BIGINT NOT NULL,
		quantity INTEGER NOT NULL DEFAULT 1,
		total_price DECIMAL(10,2) NOT NULL DEFAULT 0,
		status VARCHAR(50) NOT NULL DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT NOW(),
		updated_at TIMESTAMP DEFAULT NOW()
	);`

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}
	return nil
}
