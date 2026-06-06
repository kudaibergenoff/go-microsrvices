package handler

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zhanuzak/microservices/notification-service/internal/smtp"
	pb "github.com/zhanuzak/microservices/notification-service/proto/notification"
)

type GRPCNotificationHandler struct {
	pb.UnimplementedNotificationServiceServer
	mailer *smtp.Mailer
}

func NewGRPCNotificationHandler(mailer *smtp.Mailer) *GRPCNotificationHandler {
	return &GRPCNotificationHandler{mailer: mailer}
}

func (h *GRPCNotificationHandler) SendEmail(ctx context.Context, req *pb.SendEmailRequest) (*pb.SendEmailResponse, error) {
	if req.To == "" || req.Subject == "" {
		return nil, status.Error(codes.InvalidArgument, "to and subject are required")
	}

	if err := h.mailer.Send(req.To, req.Subject, req.Body); err != nil {
		log.Printf("ERROR: failed to send email to %s: %v", req.To, err)
		return nil, status.Error(codes.Internal, "failed to send email")
	}

	log.Printf("Email sent to %s: %s", req.To, req.Subject)
	return &pb.SendEmailResponse{Success: true}, nil
}

func (h *GRPCNotificationHandler) SendOrderNotification(ctx context.Context, req *pb.OrderNotificationRequest) (*pb.SendEmailResponse, error) {
	if req.Email == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	subject := fmt.Sprintf("Order #%d — %s", req.OrderId, req.Status)
	body := fmt.Sprintf(`
		<h2>Order Update</h2>
		<p>Your order <strong>#%d</strong> status has been updated to <strong>%s</strong>.</p>
		<p>Total: $%.2f</p>
		<p>Thank you for shopping with us!</p>
	`, req.OrderId, req.Status, req.TotalPrice)

	if err := h.mailer.Send(req.Email, subject, body); err != nil {
		log.Printf("ERROR: failed to send order notification to %s: %v", req.Email, err)
		return nil, status.Error(codes.Internal, "failed to send notification")
	}

	log.Printf("Order notification sent to %s: order #%d → %s", req.Email, req.OrderId, req.Status)
	return &pb.SendEmailResponse{Success: true}, nil
}
