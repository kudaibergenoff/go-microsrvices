package handler

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zhanuzak/microservices/user-service/internal/service"
	pb "github.com/zhanuzak/microservices/user-service/proto/user"
)

type GRPCUserHandler struct {
	pb.UnimplementedUserServiceServer
	userService service.UserService
}

func NewGRPCUserHandler(userService service.UserService) *GRPCUserHandler {
	return &GRPCUserHandler{userService: userService}
}

func (h *GRPCUserHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	user, err := h.userService.GetUser(ctx, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetUserResponse{
		Id:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		AvatarUrl: user.AvatarURL,
	}, nil
}

func (h *GRPCUserHandler) GetUserByEmail(ctx context.Context, req *pb.GetUserByEmailRequest) (*pb.GetUserResponse, error) {
	user, err := h.userService.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, toGRPCError(err)
	}

	return &pb.GetUserResponse{
		Id:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		AvatarUrl: user.AvatarURL,
	}, nil
}

func (h *GRPCUserHandler) UploadAvatar(ctx context.Context, req *pb.UploadAvatarRequest) (*pb.UploadAvatarResponse, error) {
	avatarURL, err := h.userService.UploadAvatar(ctx, req.UserId, req.Data, req.Filename, req.ContentType)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UploadAvatarResponse{AvatarUrl: avatarURL}, nil
}

func (h *GRPCUserHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	err := h.userService.UpdateUser(ctx, req.Id, req.Email, req.Name)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.UpdateUserResponse{Success: true}, nil
}

func (h *GRPCUserHandler) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	err := h.userService.DeleteUser(ctx, req.Id)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &pb.DeleteUserResponse{Success: true}, nil
}

func (h *GRPCUserHandler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users, total, err := h.userService.ListUsers(ctx, int(req.Page), int(req.Limit))
	if err != nil {
		return nil, toGRPCError(err)
	}

	pbUsers := make([]*pb.GetUserResponse, 0, len(users))
	for _, u := range users {
		pbUsers = append(pbUsers, &pb.GetUserResponse{
			Id:        u.ID,
			Email:     u.Email,
			Name:      u.Name,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			AvatarUrl: u.AvatarURL,
		})
	}

	return &pb.ListUsersResponse{
		Users: pbUsers,
		Total: int32(total),
	}, nil
}

func toGRPCError(err error) error {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, service.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrDuplicateEmail):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
