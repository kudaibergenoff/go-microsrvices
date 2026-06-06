package handler

import (
	"context"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	kafkaProducer "github.com/zhanuzak/microservices/order-service/internal/kafka"
	"github.com/zhanuzak/microservices/order-service/internal/model"
	"github.com/zhanuzak/microservices/order-service/internal/repository"
	pb "github.com/zhanuzak/microservices/order-service/proto/order"
	productpb "github.com/zhanuzak/microservices/order-service/proto/product"
)

type GRPCOrderHandler struct {
	pb.UnimplementedOrderServiceServer
	repo          repository.OrderRepository
	producer      *kafkaProducer.Producer
	productClient productpb.ProductServiceClient
}

func NewGRPCOrderHandler(repo repository.OrderRepository, producer *kafkaProducer.Producer, productClient productpb.ProductServiceClient) *GRPCOrderHandler {
	return &GRPCOrderHandler{repo: repo, producer: producer, productClient: productClient}
}

func (h *GRPCOrderHandler) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.OrderResponse, error) {
	if req.UserId == 0 || req.ProductId == 0 || req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id, product_id and quantity are required")
	}

	// SAGA Step 1: Validate product exists
	product, err := h.productClient.GetProduct(ctx, &productpb.GetProductRequest{Id: req.ProductId})
	if err != nil {
		return nil, status.Error(codes.NotFound, "product not found")
	}

	totalPrice := req.TotalPrice
	if totalPrice == 0 {
		totalPrice = product.Price * float64(req.Quantity)
	}

	// SAGA Step 2: Reserve stock
	_, err = h.productClient.ReserveStock(ctx, &productpb.ReserveStockRequest{
		ProductId: req.ProductId,
		Quantity:  req.Quantity,
	})
	if err != nil {
		log.Printf("SAGA: stock reservation failed for product %d: %v", req.ProductId, err)
		return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
	}
	log.Printf("SAGA: stock reserved — product_id=%d quantity=%d", req.ProductId, req.Quantity)

	// SAGA Step 3: Create order in DB
	order := &model.Order{
		UserID:     req.UserId,
		ProductID:  req.ProductId,
		Quantity:   int(req.Quantity),
		TotalPrice: totalPrice,
		Status:     model.StatusConfirmed,
	}

	id, err := h.repo.Create(ctx, order)
	if err != nil {
		log.Printf("SAGA: order creation failed, compensating — releasing stock for product %d", req.ProductId)
		// SAGA Compensation: Release reserved stock
		if _, relErr := h.productClient.ReleaseStock(ctx, &productpb.ReleaseStockRequest{
			ProductId: req.ProductId,
			Quantity:  req.Quantity,
		}); relErr != nil {
			log.Printf("SAGA CRITICAL: failed to release stock during compensation: %v", relErr)
		}
		return nil, status.Error(codes.Internal, "failed to create order")
	}

	order.ID = id
	log.Printf("SAGA: order created — order_id=%d status=confirmed", id)

	// Publish confirmed event
	go func() {
		if err := h.producer.PublishOrderEvent(context.Background(), model.OrderEvent{
			OrderID:    id,
			UserID:     req.UserId,
			ProductID:  req.ProductId,
			Quantity:   int(req.Quantity),
			TotalPrice: totalPrice,
			Status:     model.StatusConfirmed,
		}); err != nil {
			log.Printf("ERROR: failed to publish order.confirmed event: %v", err)
		}
	}()

	created, _ := h.repo.GetByID(ctx, id)
	if created != nil {
		order = created
	}

	return orderToProto(order), nil
}

func (h *GRPCOrderHandler) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.OrderResponse, error) {
	order, err := h.repo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}
	return orderToProto(order), nil
}

func (h *GRPCOrderHandler) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersResponse, error) {
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	}

	orders, err := h.repo.ListByUser(ctx, req.UserId, limit, int(req.Offset))
	if err != nil {
		log.Printf("ERROR: failed to list orders: %v", err)
		return nil, status.Error(codes.Internal, "failed to list orders")
	}

	pbOrders := make([]*pb.OrderResponse, 0, len(orders))
	for _, o := range orders {
		pbOrders = append(pbOrders, orderToProto(&o))
	}

	return &pb.ListOrdersResponse{
		Orders: pbOrders,
		Total:  int32(len(pbOrders)),
	}, nil
}

func (h *GRPCOrderHandler) UpdateOrderStatus(ctx context.Context, req *pb.UpdateOrderStatusRequest) (*pb.UpdateOrderStatusResponse, error) {
	s := model.OrderStatus(req.Status)
	switch s {
	case model.StatusConfirmed, model.StatusCancelled, model.StatusCompleted:
	default:
		return nil, status.Error(codes.InvalidArgument, "invalid status (confirmed, cancelled, completed)")
	}

	// Get order before update for saga compensation
	order, err := h.repo.GetByID(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.NotFound, "order not found")
	}

	// SAGA Compensation: release stock when order is cancelled
	if s == model.StatusCancelled && order.Status == model.StatusConfirmed {
		if _, relErr := h.productClient.ReleaseStock(ctx, &productpb.ReleaseStockRequest{
			ProductId: order.ProductID,
			Quantity:  int32(order.Quantity),
		}); relErr != nil {
			log.Printf("SAGA: failed to release stock on cancellation: %v", relErr)
			return nil, status.Error(codes.Internal, "failed to release stock")
		}
		log.Printf("SAGA: stock released — product_id=%d quantity=%d (order %d cancelled)", order.ProductID, order.Quantity, req.Id)
	}

	if err := h.repo.UpdateStatus(ctx, req.Id, s); err != nil {
		log.Printf("ERROR: failed to update order status: %v", err)
		return nil, status.Error(codes.Internal, "failed to update status")
	}

	go func() {
		_ = h.producer.PublishOrderEvent(context.Background(), model.OrderEvent{
			OrderID:    req.Id,
			UserID:     order.UserID,
			ProductID:  order.ProductID,
			Quantity:   order.Quantity,
			TotalPrice: order.TotalPrice,
			Status:     s,
		})
	}()

	return &pb.UpdateOrderStatusResponse{
		Success: true,
		Status:  string(s),
	}, nil
}

func orderToProto(o *model.Order) *pb.OrderResponse {
	return &pb.OrderResponse{
		Id:         o.ID,
		UserId:     o.UserID,
		ProductId:  o.ProductID,
		Quantity:   int32(o.Quantity),
		TotalPrice: o.TotalPrice,
		Status:     string(o.Status),
		CreatedAt:  o.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  o.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
