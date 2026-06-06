package handler

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/zhanuzak/microservices/user-service/internal/service"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) SetupRoutes(app *fiber.App, authMiddleware fiber.Handler) {
	users := app.Group("/api/users", authMiddleware)
	users.Get("/", h.ListUsers)
	users.Get("/:id", h.GetUser)
	users.Put("/:id", h.UpdateUser)
	users.Delete("/:id", h.DeleteUser)
}

type UpdateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	user, err := h.userService.GetUser(c.Context(), id)
	if err != nil {
		return serviceError(c, err)
	}

	return c.JSON(user)
}

func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if err := h.userService.UpdateUser(c.Context(), id, req.Email, req.Name); err != nil {
		return serviceError(c, err)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	if err := h.userService.DeleteUser(c.Context(), id); err != nil {
		return serviceError(c, err)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	users, total, err := h.userService.ListUsers(c.Context(), page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}

	return c.JSON(fiber.Map{
		"users": users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func serviceError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
	case errors.Is(err, service.ErrInvalidInput):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, service.ErrDuplicateEmail):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "email already exists"})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
	}
}
