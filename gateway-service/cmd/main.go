package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/zhanuzak/microservices/gateway-service/internal/config"
	"github.com/zhanuzak/microservices/gateway-service/internal/handler"
	"github.com/zhanuzak/microservices/gateway-service/internal/metrics"
	"github.com/zhanuzak/microservices/gateway-service/internal/middleware"
	"github.com/zhanuzak/microservices/gateway-service/internal/tracing"
	authpb "github.com/zhanuzak/microservices/gateway-service/proto/auth"
	mediapb "github.com/zhanuzak/microservices/gateway-service/proto/media"
	notifpb "github.com/zhanuzak/microservices/gateway-service/proto/notification"
	orderpb "github.com/zhanuzak/microservices/gateway-service/proto/order"
	productpb "github.com/zhanuzak/microservices/gateway-service/proto/product"
	userpb "github.com/zhanuzak/microservices/gateway-service/proto/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config: ", err)
	}

	// Initialize tracing
	shutdownTracer, err := tracing.Init(context.Background(), "gateway", cfg.JaegerEndpoint)
	if err != nil {
		log.Printf("WARNING: failed to init tracer: %v", err)
	} else {
		defer shutdownTracer(context.Background())
	}

	// gRPC connection to Auth Service (with trace propagation)
	authConn, err := grpc.NewClient(cfg.AuthGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatal("failed to connect to auth service: ", err)
	}
	defer authConn.Close()

	// gRPC connection to User Service
	userConn, err := grpc.NewClient(cfg.UserGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatal("failed to connect to user service: ", err)
	}
	defer userConn.Close()

	// gRPC connection to Product Service
	productConn, err := grpc.NewClient(cfg.ProductGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatal("failed to connect to product service: ", err)
	}
	defer productConn.Close()

	// gRPC connection to Order Service
	orderConn, err := grpc.NewClient(cfg.OrderGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatal("failed to connect to order service: ", err)
	}
	defer orderConn.Close()

	// gRPC connection to Media Service
	mediaConn, err := grpc.NewClient(cfg.MediaGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatal("failed to connect to media service: ", err)
	}
	defer mediaConn.Close()

	// gRPC connection to Notification Service
	notifConn, err := grpc.NewClient(cfg.NotificationGRPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatal("failed to connect to notification service: ", err)
	}
	defer notifConn.Close()

	authClient := authpb.NewAuthServiceClient(authConn)
	userClient := userpb.NewUserServiceClient(userConn)
	productClient := productpb.NewProductServiceClient(productConn)
	orderClient := orderpb.NewOrderServiceClient(orderConn)
	mediaClient := mediapb.NewMediaServiceClient(mediaConn)
	notifClient := notifpb.NewNotificationServiceClient(notifConn)

	app := fiber.New(fiber.Config{
		AppName: "API Gateway",
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "{\"time\":\"${time}\",\"status\":${status},\"latency\":\"${latency}\",\"ip\":\"${ip}\",\"method\":\"${method}\",\"path\":\"${path}\"}\n",
		TimeFormat: "2006-01-02T15:04:05Z07:00",
	}))
	app.Use(cors.New())
	log.Printf("  Rate limit → %d req / %ds", cfg.RateLimitMax, cfg.RateLimitWindow)
	app.Use(limiter.New(limiter.Config{
		Max:        cfg.RateLimitMax,
		Expiration: time.Duration(cfg.RateLimitWindow) * time.Second,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			c.Set("Retry-After", "60")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests",
			})
		},
		SkipSuccessfulRequests: false,
	}))
	app.Use(metrics.Middleware())
	app.Use(tracing.Middleware("gateway"))

	// Metrics & health
	app.Get("/metrics", metrics.Handler())
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "gateway"})
	})

	// Auth routes — public
	authHandler := handler.NewAuthHandler(authClient)
	authHandler.SetupRoutes(app)

	// User routes — protected with Keycloak JWT validation
	keycloakMiddleware := middleware.KeycloakAuth(cfg.KeycloakIssuerURL())
	adminMiddleware := middleware.RequireRole("admin")
	userHandler := handler.NewUserHandler(userClient, authClient)
	userHandler.SetupRoutes(app, keycloakMiddleware, adminMiddleware)

	// Product routes — public
	productHandler := handler.NewProductHandler(productClient)
	productHandler.SetupRoutes(app)

	// Order routes — public
	orderHandler := handler.NewOrderHandler(orderClient)
	orderHandler.SetupRoutes(app)

	// Media routes — public
	mediaHandler := handler.NewMediaHandler(mediaClient)
	mediaHandler.SetupRoutes(app)

	// Notification routes — public
	notifHandler := handler.NewNotificationHandler(notifClient)
	notifHandler.SetupRoutes(app)

	go func() {
		log.Printf("API Gateway listening on :%s", cfg.AppPort)
		log.Printf("  Auth gRPC    → %s", cfg.AuthGRPCAddr())
		log.Printf("  User gRPC    → %s", cfg.UserGRPCAddr())
		log.Printf("  Product gRPC → %s", cfg.ProductGRPCAddr())
		log.Printf("  Order gRPC   → %s", cfg.OrderGRPCAddr())
		log.Printf("  Media gRPC   → %s", cfg.MediaGRPCAddr())
		log.Printf("  Notif gRPC   → %s", cfg.NotificationGRPCAddr())
		log.Printf("  Keycloak     → %s", cfg.KeycloakIssuerURL())
		if err := app.Listen(":" + cfg.AppPort); err != nil {
			log.Fatal("failed to start gateway: ", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down gateway...")
	_ = app.Shutdown()
	log.Println("gateway stopped")
}
