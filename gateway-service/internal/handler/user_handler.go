package handler

import (
	"io"
	"strconv"

	"github.com/gofiber/fiber/v2"

	authpb "github.com/zhanuzak/microservices/gateway-service/proto/auth"
	userpb "github.com/zhanuzak/microservices/gateway-service/proto/user"
)

type UserHandler struct {
	userClient userpb.UserServiceClient
	authClient authpb.AuthServiceClient
}

func NewUserHandler(userClient userpb.UserServiceClient, authClient authpb.AuthServiceClient) *UserHandler {
	return &UserHandler{userClient: userClient, authClient: authClient}
}

func (h *UserHandler) SetupRoutes(app *fiber.App, authMiddleware fiber.Handler, adminMiddleware fiber.Handler) {
	users := app.Group("/api/users", authMiddleware)
	// /me routes — any authenticated user
	users.Get("/me", h.GetMe)
	users.Put("/me", h.UpdateMe)
	users.Put("/me/password", h.ChangePassword)
	users.Post("/me/avatar", h.UploadAvatar)
	users.Delete("/me", h.DeleteMe)
	// Admin-only routes
	users.Get("/", adminMiddleware, h.ListUsers)
	users.Get("/:id", adminMiddleware, h.GetUser)
	users.Put("/:id", adminMiddleware, h.UpdateUser)
	users.Delete("/:id", adminMiddleware, h.DeleteUser)
}

type UpdateUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

type ChangePasswordRequest struct {
	NewPassword string `json:"new_password"`
}

func (h *UserHandler) GetMe(c *fiber.Ctx) error {
	email := c.Get("X-User-Email")
	if email == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "email not found in token"})
	}

	resp, err := h.userClient.GetUserByEmail(c.Context(), &userpb.GetUserByEmailRequest{Email: email})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":         resp.Id,
		"email":      resp.Email,
		"name":       resp.Name,
		"avatar_url": resp.AvatarUrl,
		"created_at": resp.CreatedAt,
	})
}

func (h *UserHandler) UpdateMe(c *fiber.Ctx) error {
	email := c.Get("X-User-Email")
	if email == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "email not found in token"})
	}

	// Get current user to find ID
	user, err := h.userClient.GetUserByEmail(c.Context(), &userpb.GetUserByEmailRequest{Email: email})
	if err != nil {
		return grpcError(c, err)
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	_, err = h.userClient.UpdateUser(c.Context(), &userpb.UpdateUserRequest{
		Id:    user.Id,
		Email: req.Email,
		Name:  req.Name,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	email := c.Get("X-User-Email")
	if email == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "email not found in token"})
	}

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "new_password is required"})
	}

	_, err := h.authClient.ChangePassword(c.Context(), &authpb.ChangePasswordRequest{
		Email:       email,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *UserHandler) UploadAvatar(c *fiber.Ctx) error {
	email := c.Get("X-User-Email")
	if email == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "email not found in token"})
	}

	user, err := h.userClient.GetUserByEmail(c.Context(), &userpb.GetUserByEmailRequest{Email: email})
	if err != nil {
		return grpcError(c, err)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}

	resp, err := h.userClient.UploadAvatar(c.Context(), &userpb.UploadAvatarRequest{
		UserId:      user.Id,
		Data:        data,
		Filename:    file.Filename,
		ContentType: file.Header.Get("Content-Type"),
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"avatar_url": resp.AvatarUrl})
}

func (h *UserHandler) DeleteMe(c *fiber.Ctx) error {
	email := c.Get("X-User-Email")
	if email == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "email not found in token"})
	}

	user, err := h.userClient.GetUserByEmail(c.Context(), &userpb.GetUserByEmailRequest{Email: email})
	if err != nil {
		return grpcError(c, err)
	}

	_, err = h.userClient.DeleteUser(c.Context(), &userpb.DeleteUserRequest{Id: user.Id})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	resp, err := h.userClient.GetUser(c.Context(), &userpb.GetUserRequest{Id: id})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":         resp.Id,
		"email":      resp.Email,
		"name":       resp.Name,
		"avatar_url": resp.AvatarUrl,
		"created_at": resp.CreatedAt,
	})
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

	_, err = h.userClient.UpdateUser(c.Context(), &userpb.UpdateUserRequest{
		Id:    id,
		Email: req.Email,
		Name:  req.Name,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user id"})
	}

	_, err = h.userClient.DeleteUser(c.Context(), &userpb.DeleteUserRequest{Id: id})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"success": true})
}

func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	resp, err := h.userClient.ListUsers(c.Context(), &userpb.ListUsersRequest{
		Page:  int32(page),
		Limit: int32(limit),
	})
	if err != nil {
		return grpcError(c, err)
	}

	users := make([]fiber.Map, 0, len(resp.Users))
	for _, u := range resp.Users {
		users = append(users, fiber.Map{
			"id":         u.Id,
			"email":      u.Email,
			"name":       u.Name,
			"avatar_url": u.AvatarUrl,
			"created_at": u.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{
		"users": users,
		"total": resp.Total,
		"page":  page,
		"limit": limit,
	})
}
