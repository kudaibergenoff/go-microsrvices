package handler

import (
	"context"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zhanuzak/microservices/product-service/internal/model"
	"github.com/zhanuzak/microservices/product-service/internal/repository"
	pb "github.com/zhanuzak/microservices/product-service/proto/product"
)

type GRPCProductHandler struct {
	pb.UnimplementedProductServiceServer
	repo repository.ProductRepository
}

func NewGRPCProductHandler(repo repository.ProductRepository) *GRPCProductHandler {
	return &GRPCProductHandler{repo: repo}
}

func (h *GRPCProductHandler) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.ProductResponse, error) {
	if req.Name == "" || req.Price <= 0 {
		return nil, status.Error(codes.InvalidArgument, "name and price are required")
	}

	product := &model.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImageURL:    req.ImageUrl,
		Stock:       int(req.Stock),
	}

	id, err := h.repo.Create(ctx, product)
	if err != nil {
		log.Printf("ERROR: failed to create product: %v", err)
		return nil, status.Error(codes.Internal, "failed to create product")
	}

	product.ID = id
	created, _ := h.repo.GetByID(ctx, id)
	if created != nil {
		product = created
	}

	return productToProto(product), nil
}

func (h *GRPCProductHandler) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.ProductResponse, error) {
	product, err := h.repo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "product not found")
	}
	return productToProto(product), nil
}

func (h *GRPCProductHandler) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	products, err := h.repo.List(ctx, req.Category, limit, int(req.Offset))
	if err != nil {
		log.Printf("ERROR: failed to list products: %v", err)
		return nil, status.Error(codes.Internal, "failed to list products")
	}

	pbProducts := make([]*pb.ProductResponse, 0, len(products))
	for _, p := range products {
		pbProducts = append(pbProducts, productToProto(&p))
	}

	return &pb.ListProductsResponse{
		Products: pbProducts,
		Total:    int32(len(pbProducts)),
	}, nil
}

func (h *GRPCProductHandler) UpdateProduct(ctx context.Context, req *pb.UpdateProductRequest) (*pb.UpdateProductResponse, error) {
	product := &model.Product{
		ID:          req.Id,
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Category:    req.Category,
		ImageURL:    req.ImageUrl,
		Stock:       int(req.Stock),
	}

	if err := h.repo.Update(ctx, product); err != nil {
		log.Printf("ERROR: failed to update product: %v", err)
		return nil, status.Error(codes.Internal, "failed to update product")
	}

	return &pb.UpdateProductResponse{Success: true}, nil
}

func (h *GRPCProductHandler) DeleteProduct(ctx context.Context, req *pb.DeleteProductRequest) (*pb.DeleteProductResponse, error) {
	if err := h.repo.Delete(ctx, req.Id); err != nil {
		log.Printf("ERROR: failed to delete product: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete product")
	}
	return &pb.DeleteProductResponse{Success: true}, nil
}

func (h *GRPCProductHandler) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.ListProductsResponse, error) {
	if req.Query == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	products, err := h.repo.Search(ctx, req.Query, limit)
	if err != nil {
		log.Printf("ERROR: search failed: %v", err)
		return nil, status.Error(codes.Internal, "search failed")
	}

	pbProducts := make([]*pb.ProductResponse, 0, len(products))
	for _, p := range products {
		pbProducts = append(pbProducts, productToProto(&p))
	}

	return &pb.ListProductsResponse{
		Products: pbProducts,
		Total:    int32(len(pbProducts)),
	}, nil
}

func (h *GRPCProductHandler) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	if req.ProductId == 0 || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "product_id and quantity are required")
	}

	if err := h.repo.ReserveStock(ctx, req.ProductId, int(req.Quantity)); err != nil {
		log.Printf("ERROR: failed to reserve stock: %v", err)
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}

	log.Printf("Reserved %d units for product %d", req.Quantity, req.ProductId)
	return &pb.ReserveStockResponse{Success: true}, nil
}

func (h *GRPCProductHandler) ReleaseStock(ctx context.Context, req *pb.ReleaseStockRequest) (*pb.ReleaseStockResponse, error) {
	if req.ProductId == 0 || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "product_id and quantity are required")
	}

	if err := h.repo.ReleaseStock(ctx, req.ProductId, int(req.Quantity)); err != nil {
		log.Printf("ERROR: failed to release stock: %v", err)
		return nil, status.Error(codes.Internal, "failed to release stock")
	}

	log.Printf("Released %d units for product %d", req.Quantity, req.ProductId)
	return &pb.ReleaseStockResponse{Success: true}, nil
}

func productToProto(p *model.Product) *pb.ProductResponse {
	return &pb.ProductResponse{
		Id:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		Category:    p.Category,
		ImageUrl:    p.ImageURL,
		Stock:       int32(p.Stock),
		CreatedAt:   p.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
