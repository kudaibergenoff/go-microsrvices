package handler

import (
	"io"

	"github.com/gofiber/fiber/v2"

	mediapb "github.com/zhanuzak/microservices/gateway-service/proto/media"
)

type MediaHandler struct {
	mediaClient mediapb.MediaServiceClient
}

func NewMediaHandler(mediaClient mediapb.MediaServiceClient) *MediaHandler {
	return &MediaHandler{mediaClient: mediaClient}
}

func (h *MediaHandler) SetupRoutes(app *fiber.App) {
	media := app.Group("/api/media")
	media.Post("/upload", h.Upload)
	media.Delete("/:key", h.Delete)
	media.Get("/url/:key", h.GetURL)
}

func (h *MediaHandler) Upload(c *fiber.Ctx) error {
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

	folder := c.FormValue("folder", "uploads")

	resp, err := h.mediaClient.UploadFile(c.Context(), &mediapb.UploadFileRequest{
		Data:        data,
		Filename:    file.Filename,
		ContentType: file.Header.Get("Content-Type"),
		Folder:      folder,
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"key": resp.Key,
		"url": resp.Url,
	})
}

func (h *MediaHandler) Delete(c *fiber.Ctx) error {
	key := c.Params("key")
	folder := c.Query("folder", "uploads")
	fullKey := folder + "/" + key

	_, err := h.mediaClient.DeleteFile(c.Context(), &mediapb.DeleteFileRequest{Key: fullKey})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"message": "file deleted"})
}

func (h *MediaHandler) GetURL(c *fiber.Ctx) error {
	key := c.Params("key")
	folder := c.Query("folder", "uploads")
	fullKey := folder + "/" + key

	resp, err := h.mediaClient.GetFileURL(c.Context(), &mediapb.GetFileURLRequest{Key: fullKey})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"url": resp.Url})
}
