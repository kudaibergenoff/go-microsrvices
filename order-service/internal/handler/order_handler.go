package handler

import (
	"context"
	"log"
	"strconv"

	"github.com/gofiber/fiber/v2"

	kafkaProducer "github.com/zhanuzak/microservices/order-service/internal/kafka"
	"github.com/zhanuzak/microservices/order-service/internal/model"
	"github.com/zhanuzak/microservices/order-service/internal/repository"
)

type OrderHandler struct {
	repo     repository.OrderRepository
	producer *kafkaProducer.Producer
}

func NewOrderHandler(repo repository.OrderRepository, producer *kafkaProducer.Producer) *OrderHandler {
	return &OrderHandler{repo: repo, producer: producer}
}

func (h *OrderHandler) SetupRoutes(app *fiber.App) {
	orders := app.Group("/api/orders")
	orders.Post("/", h.Create)
	orders.Get("/", h.ListByUser)
	orders.Get("/:id", h.GetByID)
	orders.Put("/:id/status", h.UpdateStatus)
}

type CreateOrderRequest struct {
	UserID     int64   `json:"user_id"`
	ProductID  int64   `json:"product_id"`
	Quantity   int     `json:"quantity"`
	TotalPrice float64 `json:"total_price"`
}

type UpdateStatusRequest struct {
	Status string `json:"status"`
}

func (h *OrderHandler) Create(c *fiber.Ctx) error {
	var req CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.UserID == 0 || req.ProductID == 0 || req.Quantity <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id, product_id and quantity are required"})
	}

	order := &model.Order{
		UserID:     req.UserID,
		ProductID:  req.ProductID,
		Quantity:   req.Quantity,
		TotalPrice: req.TotalPrice,
		Status:     model.StatusPending,
	}

	id, err := h.repo.Create(c.Context(), order)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create order"})
	}
	order.ID = id

	go func() {
		if err := h.producer.PublishOrderEvent(context.Background(), model.OrderEvent{
			OrderID:    id,
			UserID:     req.UserID,
			ProductID:  req.ProductID,
			Quantity:   req.Quantity,
			TotalPrice: req.TotalPrice,
			Status:     model.StatusPending,
		}); err != nil {
			log.Printf("ERROR: failed to publish order.created event: %v", err)
		}
	}()

	return c.Status(fiber.StatusCreated).JSON(order)
}

func (h *OrderHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	order, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "order not found"})
	}
	return c.JSON(order)
}

func (h *OrderHandler) ListByUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id query param is required"})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	orders, err := h.repo.ListByUser(c.Context(), userID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list orders"})
	}

	return c.JSON(fiber.Map{"orders": orders})
}

func (h *OrderHandler) UpdateStatus(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var req UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	status := model.OrderStatus(req.Status)
	switch status {
	case model.StatusConfirmed, model.StatusCancelled, model.StatusCompleted:
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid status (confirmed, cancelled, completed)"})
	}

	if err := h.repo.UpdateStatus(c.Context(), id, status); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update status"})
	}

	go func() {
		order, err := h.repo.GetByID(context.Background(), id)
		if err != nil {
			return
		}
		_ = h.producer.PublishOrderEvent(context.Background(), model.OrderEvent{
			OrderID:    id,
			UserID:     order.UserID,
			ProductID:  order.ProductID,
			Quantity:   order.Quantity,
			TotalPrice: order.TotalPrice,
			Status:     status,
		})
	}()

	return c.JSON(fiber.Map{"message": "status updated", "status": status})
}
