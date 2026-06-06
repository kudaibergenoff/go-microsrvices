package handler

import (
	"context"
	"errors"
	"log"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zhanuzak/microservices/auth-service/internal/service"
	pb "github.com/zhanuzak/microservices/auth-service/proto/auth"
)

type GRPCAuthHandler struct {
	pb.UnimplementedAuthServiceServer
	authService service.AuthService
}

func NewGRPCAuthHandler(authService service.AuthService) *GRPCAuthHandler {
	return &GRPCAuthHandler{authService: authService}
}

func (h *GRPCAuthHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	userID, token, refreshToken, err := h.authService.Register(ctx, req.Email, req.Password, req.Name)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.RegisterResponse{
		UserId:       userID,
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (h *GRPCAuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	userID, token, refreshToken, err := h.authService.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.LoginResponse{
		UserId:       userID,
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (h *GRPCAuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	token, refreshToken, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.RefreshTokenResponse{
		Token:        token,
		RefreshToken: refreshToken,
	}, nil
}

func (h *GRPCAuthHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	userID, email, err := h.authService.ValidateToken(ctx, req.Token)
	if err != nil {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	return &pb.ValidateTokenResponse{
		Valid:  true,
		UserId: userID,
		Email:  email,
	}, nil
}

func (h *GRPCAuthHandler) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*pb.ChangePasswordResponse, error) {
	if err := h.authService.ChangePassword(ctx, req.Email, req.NewPassword); err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.ChangePasswordResponse{Success: true}, nil
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, service.ErrUserAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, service.ErrInvalidToken):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, service.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		log.Printf("ERROR: unhandled error: %v", err)
		return status.Error(codes.Internal, "internal server error")
	}
}
