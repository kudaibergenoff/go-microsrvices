package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"

	"github.com/zhanuzak/microservices/user-service/internal/model"
	"github.com/zhanuzak/microservices/user-service/internal/repository"
	mediapb "github.com/zhanuzak/microservices/user-service/proto/media"
)

var (
	ErrUserNotFound   = errors.New("user not found")
	ErrInvalidInput   = errors.New("invalid input")
	ErrDuplicateEmail = errors.New("email already exists")
)

type UserService interface {
	GetUser(ctx context.Context, id int64) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	UpdateUser(ctx context.Context, id int64, email, name string) error
	UploadAvatar(ctx context.Context, userID int64, data []byte, filename, contentType string) (string, error)
	DeleteUser(ctx context.Context, id int64) error
	ListUsers(ctx context.Context, page, limit int) ([]*model.User, int, error)
}

type userService struct {
	repo        repository.UserRepository
	mediaClient mediapb.MediaServiceClient
}

func NewUserService(repo repository.UserRepository, mediaClient mediapb.MediaServiceClient) UserService {
	return &userService{repo: repo, mediaClient: mediaClient}
}

func (s *userService) GetUser(ctx context.Context, id int64) (*model.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *userService) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (s *userService) UpdateUser(ctx context.Context, id int64, email, name string) error {
	if email == "" && name == "" {
		return fmt.Errorf("%w: email or name must be provided", ErrInvalidInput)
	}

	if email != "" {
		if _, err := mail.ParseAddress(email); err != nil {
			return fmt.Errorf("%w: invalid email format", ErrInvalidInput)
		}
	}

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("get user for update: %w", err)
	}

	if email != "" {
		user.Email = email
	}
	if name != "" {
		user.Name = name
	}

	if err := s.repo.Update(ctx, user); err != nil {
		if errors.Is(err, repository.ErrDuplicateEmail) {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("update user: %w", err)
	}

	return nil
}

func (s *userService) UploadAvatar(ctx context.Context, userID int64, data []byte, filename, contentType string) (string, error) {
	// Verify user exists
	if _, err := s.repo.GetByID(ctx, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrUserNotFound
		}
		return "", fmt.Errorf("get user for avatar: %w", err)
	}

	// Upload to media service via gRPC
	resp, err := s.mediaClient.UploadFile(ctx, &mediapb.UploadFileRequest{
		Data:        data,
		Filename:    filename,
		ContentType: contentType,
		Folder:      "avatars",
	})
	if err != nil {
		return "", fmt.Errorf("upload avatar to media service: %w", err)
	}

	// Save avatar_url in DB
	if err := s.repo.UpdateAvatarURL(ctx, userID, resp.Url); err != nil {
		return "", fmt.Errorf("update avatar url: %w", err)
	}

	return resp.Url, nil
}

func (s *userService) DeleteUser(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *userService) ListUsers(ctx context.Context, page, limit int) ([]*model.User, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	return s.repo.List(ctx, page, limit)
}
