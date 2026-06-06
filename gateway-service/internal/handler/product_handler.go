package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	productpb "github.com/zhanuzak/microservices/gateway-service/proto/product"
)

type ProductHandler struct {
	productClient productpb.ProductServiceClient
}

func NewProductHandler(productClient productpb.ProductServiceClient) *ProductHandler {
	return &ProductHandler{productClient: productClient}
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

	resp, err := h.productClient.CreateProduct(c.Context(), &productpb.CreateProductRequest{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImageUrl:    req.ImageURL,
		Stock:       int32(req.Stock),
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":          resp.Id,
		"name":        resp.Name,
		"description": resp.Description,
		"price":       resp.Price,
		"category":    resp.Category,
		"image_url":   resp.ImageUrl,
		"stock":       resp.Stock,
		"created_at":  resp.CreatedAt,
		"updated_at":  resp.UpdatedAt,
	})
}

func (h *ProductHandler) GetByID(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	resp, err := h.productClient.GetProduct(c.Context(), &productpb.GetProductRequest{Id: id})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{
		"id":          resp.Id,
		"name":        resp.Name,
		"description": resp.Description,
		"price":       resp.Price,
		"category":    resp.Category,
		"image_url":   resp.ImageUrl,
		"stock":       resp.Stock,
		"created_at":  resp.CreatedAt,
		"updated_at":  resp.UpdatedAt,
	})
}

func (h *ProductHandler) List(c *fiber.Ctx) error {
	category := c.Query("category")
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	resp, err := h.productClient.ListProducts(c.Context(), &productpb.ListProductsRequest{
		Category: category,
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
	if err != nil {
		return grpcError(c, err)
	}

	products := make([]fiber.Map, 0, len(resp.Products))
	for _, p := range resp.Products {
		products = append(products, fiber.Map{
			"id":          p.Id,
			"name":        p.Name,
			"description": p.Description,
			"price":       p.Price,
			"category":    p.Category,
			"image_url":   p.ImageUrl,
			"stock":       p.Stock,
			"created_at":  p.CreatedAt,
			"updated_at":  p.UpdatedAt,
		})
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

	resp, err := h.productClient.SearchProducts(c.Context(), &productpb.SearchProductsRequest{
		Query: query,
		Limit: int32(limit),
	})
	if err != nil {
		return grpcError(c, err)
	}

	products := make([]fiber.Map, 0, len(resp.Products))
	for _, p := range resp.Products {
		products = append(products, fiber.Map{
			"id":          p.Id,
			"name":        p.Name,
			"description": p.Description,
			"price":       p.Price,
			"category":    p.Category,
			"image_url":   p.ImageUrl,
			"stock":       p.Stock,
			"created_at":  p.CreatedAt,
			"updated_at":  p.UpdatedAt,
		})
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

	_, err = h.productClient.UpdateProduct(c.Context(), &productpb.UpdateProductRequest{
		Id:          id,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImageUrl:    req.ImageURL,
		Stock:       int32(req.Stock),
	})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"message": "product updated"})
}

func (h *ProductHandler) Delete(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid id"})
	}

	_, err = h.productClient.DeleteProduct(c.Context(), &productpb.DeleteProductRequest{Id: id})
	if err != nil {
		return grpcError(c, err)
	}

	return c.JSON(fiber.Map{"message": "product deleted"})
}
