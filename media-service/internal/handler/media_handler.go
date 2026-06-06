package handler

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/zhanuzak/microservices/media-service/internal/storage"
)

var allowedTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

const maxFileSize = 10 << 20 // 10 MB

type MediaHandler struct {
	storage *storage.MinioStorage
}

func NewMediaHandler(storage *storage.MinioStorage) *MediaHandler {
	return &MediaHandler{storage: storage}
}

func (h *MediaHandler) SetupRoutes(app *fiber.App) {
	media := app.Group("/api/media")
	media.Post("/upload", h.Upload)
	media.Delete("/:key", h.Delete)
}

func (h *MediaHandler) Upload(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	if file.Size > maxFileSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file too large (max 10MB)"})
	}

	contentType := file.Header.Get("Content-Type")
	if !allowedTypes[contentType] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("unsupported file type: %s (allowed: jpeg, png, gif, webp)", contentType),
		})
	}

	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to read file"})
	}
	defer src.Close()

	// Generate unique key: folder/uuid.ext
	folder := c.FormValue("folder", "uploads")
	ext := strings.ToLower(filepath.Ext(file.Filename))
	key := fmt.Sprintf("%s/%s%s", folder, uuid.New().String(), ext)

	url, err := h.storage.Upload(c.Context(), key, src, file.Size, contentType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to upload file"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"key": key,
		"url": url,
	})
}

func (h *MediaHandler) Delete(c *fiber.Ctx) error {
	key := c.Params("key")
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "key is required"})
	}

	// Support nested keys via query param
	folder := c.Query("folder", "uploads")
	fullKey := fmt.Sprintf("%s/%s", folder, key)

	if err := h.storage.Delete(c.Context(), fullKey); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete file"})
	}

	return c.JSON(fiber.Map{"message": "file deleted"})
}
