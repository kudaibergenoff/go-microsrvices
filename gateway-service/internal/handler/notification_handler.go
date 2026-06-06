package handler

import (
	"github.com/gofiber/fiber/v2"

	notifpb "github.com/zhanuzak/microservices/gateway-service/proto/notification"
)

type NotificationHandler struct {
	notifClient notifpb.NotificationServiceClient
}

func NewNotificationHandler(notifClient notifpb.NotificationServiceClient) *NotificationHandler {
	return &NotificationHandler{notifClient: notifClient}
}

func (h *NotificationHandler) SetupRoutes(app *fiber.App) {
	notif := app.Group("/api/notifications")
	notif.Post("/email", h.SendEmail)
	notif.Post("/order", h.SendOrderNotification)
}

type SendEmailRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type OrderNotifRequest struct {
	Email      string  `json:"email"`
	OrderID    int64   `json:"order_id"`
	Status     string  `json:"status"`
	TotalPrice float64 `json:"total_price"`
}

func (h *NotificationHandler) SendEmail(c *fiber.Ctx) error {
	var req SendEmailRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	resp, err := h.notifClient.SendEmail(c.Context(), &notifpb.SendEmailRequest{
		To:      req.To,
		Subject: req.Subject,
		Body:    req.Body,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"success": resp.Success})
}

func (h *NotificationHandler) SendOrderNotification(c *fiber.Ctx) error {
	var req OrderNotifRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	resp, err := h.notifClient.SendOrderNotification(c.Context(), &notifpb.OrderNotificationRequest{
		Email:      req.Email,
		OrderId:    req.OrderID,
		Status:     req.Status,
		TotalPrice: req.TotalPrice,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"success": resp.Success})
}
