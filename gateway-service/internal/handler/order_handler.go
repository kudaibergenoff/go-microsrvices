package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	orderpb "github.com/zhanuzak/microservices/gateway-service/proto/order"
)

type OrderHandler struct {
	orderClient orderpb.OrderServiceClient
}

func NewOrderHandler(orderClient orderpb.OrderServiceClient) *OrderHandler {
	return &OrderHandler{orderClient: orderClient}
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

	resp, err := h.orderClient.CreateOrder(c.Context(), &orderpb.CreateOrderRequest{
		UserId:     req.UserID,
		ProductId:  req.ProductID,
		Quantity:   int32(req.Quantity),
		TotalPrice: req.TotalPrice,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":          resp.Id,
		"user_id":     resp.UserId,
		"product_id":  resp.ProductId,
		"quantity":    resp.Quantity,
		"total_price": resp.TotalPrice,
		"status":      resp.Status,
		"created_at":  resp.CreatedAt,
		"updated_at":  resp.UpdatedAt,
	})
}

func (h *OrderHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	resp, err := h.orderClient.GetOrder(c.Context(), &orderpb.GetOrderRequest{Id: id})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":          resp.Id,
		"user_id":     resp.UserId,
		"product_id":  resp.ProductId,
		"quantity":    resp.Quantity,
		"total_price": resp.TotalPrice,
		"status":      resp.Status,
		"created_at":  resp.CreatedAt,
		"updated_at":  resp.UpdatedAt,
	})
}

func (h *OrderHandler) ListByUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseInt(c.Query("user_id"), 10, 64)
	if err != nil || userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id query param is required"})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	resp, err := h.orderClient.ListOrders(c.Context(), &orderpb.ListOrdersRequest{
		UserId: userID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return grpcError(c, err)
	}

	orders := make([]fiber.Map, 0, len(resp.Orders))
	for _, o := range resp.Orders {
		orders = append(orders, fiber.Map{
			"id":          o.Id,
			"user_id":     o.UserId,
			"product_id":  o.ProductId,
			"quantity":    o.Quantity,
			"total_price": o.TotalPrice,
			"status":      o.Status,
			"created_at":  o.CreatedAt,
			"updated_at":  o.UpdatedAt,
		})
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

	resp, err := h.orderClient.UpdateOrderStatus(c.Context(), &orderpb.UpdateOrderStatusRequest{
		Id:     id,
		Status: req.Status,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"message": "status updated", "status": resp.Status})
}
