package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/zhanuzak/microservices/product-service/internal/model"
	"github.com/zhanuzak/microservices/product-service/internal/repository"
)

type ProductHandler struct {
	repo repository.ProductRepository
}

func NewProductHandler(repo repository.ProductRepository) *ProductHandler {
	return &ProductHandler{repo: repo}
}

func (h *ProductHandler) SetupRoutes(app *fiber.App) {
	products := app.Group("/api/products")
	products.Post("/", h.Create)
	products.Get("/", h.List)
	products.Get("/search", h.Search)
	products.Get("/:id", h.GetByID)
	products.Put("/:id", h.Update)
	products.Delete("/:id", h.Delete)
}

type CreateProductRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	ImageURL    string  `json:"image_url"`
	Stock       int     `json:"stock"`
}

func (h *ProductHandler) Create(c *fiber.Ctx) error {
	var req CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Name == "" || req.Price <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name and price are required"})
	}

	product := &model.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImageURL:    req.ImageURL,
		Stock:       req.Stock,
	}

	id, err := h.repo.Create(c.Context(), product)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create product"})
	}

	product.ID = id
	return c.Status(fiber.StatusCreated).JSON(product)
}

func (h *ProductHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	product, err := h.repo.GetByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "product not found"})
	}

	return c.JSON(product)
}

func (h *ProductHandler) List(c *fiber.Ctx) error {
	category := c.Query("category")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	if limit > 100 {
		limit = 100
	}

	products, err := h.repo.List(c.Context(), category, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list products"})
	}

	return c.JSON(fiber.Map{
		"products": products,
		"limit":    limit,
		"offset":   offset,
	})
}

func (h *ProductHandler) Search(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "query parameter 'q' is required"})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	products, err := h.repo.Search(c.Context(), query, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "search failed"})
	}

	return c.JSON(fiber.Map{"products": products, "query": query})
}

func (h *ProductHandler) Update(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	var req CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	product := &model.Product{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImageURL:    req.ImageURL,
		Stock:       req.Stock,
	}

	if err := h.repo.Update(c.Context(), product); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update product"})
	}

	return c.JSON(fiber.Map{"message": "product updated"})
}

func (h *ProductHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	if err := h.repo.Delete(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete product"})
	}

	return c.JSON(fiber.Map{"message": "product deleted"})
}
