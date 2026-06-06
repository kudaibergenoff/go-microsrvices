package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/zhanuzak/microservices/auth-service/internal/service"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
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

type AuthResponse struct {
	UserID       int64  `json:"user_id"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Email == "" || req.Password == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email, password and name are required"})
	}

	userID, token, refreshToken, err := h.authService.Register(c.Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "user already exists"})
		}
		if errors.Is(err, service.ErrInvalidInput) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.Status(fiber.StatusCreated).JSON(AuthResponse{UserID: userID, Token: token, RefreshToken: refreshToken})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email and password are required"})
	}

	userID, token, refreshToken, err := h.authService.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
		}
		if errors.Is(err, service.ErrInvalidInput) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.JSON(AuthResponse{UserID: userID, Token: token, RefreshToken: refreshToken})
}

func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "refresh_token is required"})
	}

	token, refreshToken, err := h.authService.RefreshToken(c.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired refresh token"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.JSON(fiber.Map{
		"token":         token,
		"refresh_token": refreshToken,
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

	userID, email, err := h.authService.ValidateToken(c.Context(), req.Token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid token"})
	}

	return c.JSON(fiber.Map{
		"valid":   true,
		"user_id": userID,
		"email":   email,
	})
}
