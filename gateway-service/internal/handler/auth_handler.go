package handler

import (
	"github.com/gofiber/fiber/v2"

	pb "github.com/zhanuzak/microservices/gateway-service/proto/auth"
)

type AuthHandler struct {
	authClient pb.AuthServiceClient
}

func NewAuthHandler(authClient pb.AuthServiceClient) *AuthHandler {
	return &AuthHandler{authClient: authClient}
}

func (h *AuthHandler) SetupRoutes(app *fiber.App) {
	auth := app.Group("/api/auth")
	auth.Post("/register", h.Register)
	auth.Post("/login", h.Login)
	auth.Post("/refresh", h.RefreshToken)
	auth.Post("/validate", h.ValidateToken)
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type ValidateRequest struct {
	Token string `json:"token"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email, password and name are required"})
	}

	resp, err := h.authClient.Register(c.Context(), &pb.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"user_id":       resp.UserId,
		"token":         resp.Token,
		"refresh_token": resp.RefreshToken,
	})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email and password are required"})
	}

	resp, err := h.authClient.Login(c.Context(), &pb.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{
		"user_id":       resp.UserId,
		"token":         resp.Token,
		"refresh_token": resp.RefreshToken,
	})
}

func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "refresh_token is required"})
	}

	resp, err := h.authClient.RefreshToken(c.Context(), &pb.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{
		"token":         resp.Token,
		"refresh_token": resp.RefreshToken,
	})
}

func (h *AuthHandler) ValidateToken(c *fiber.Ctx) error {
	var req ValidateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token is required"})
	}

	resp, err := h.authClient.ValidateToken(c.Context(), &pb.ValidateTokenRequest{
		Token: req.Token,
	})
	if err != nil || !resp.Valid {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}

	return c.JSON(fiber.Map{
		"valid":   resp.Valid,
		"user_id": resp.UserId,
		"email":   resp.Email,
	})
}
