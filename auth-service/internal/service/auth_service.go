package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/mail"
	"strings"

	kafkaProducer "github.com/zhanuzak/microservices/auth-service/internal/kafka"
	"github.com/zhanuzak/microservices/auth-service/internal/keycloak"
	"github.com/zhanuzak/microservices/auth-service/internal/model"
	"github.com/zhanuzak/microservices/auth-service/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserAlreadyExists  = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidInput       = errors.New("invalid input")
)

const minPasswordLength = 6

type AuthService interface {
	Register(ctx context.Context, email, password, name string) (int64, string, string, error)
	Login(ctx context.Context, email, password string) (int64, string, string, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, string, error)
	ValidateToken(ctx context.Context, token string) (int64, string, error)
	ChangePassword(ctx context.Context, email, newPassword string) error
}

type authService struct {
	repo     repository.UserRepository
	producer *kafkaProducer.Producer
	kc       *keycloak.Client
}

func NewAuthService(repo repository.UserRepository, producer *kafkaProducer.Producer, kc *keycloak.Client) AuthService {
	return &authService{
		repo:     repo,
		producer: producer,
		kc:       kc,
	}
}

func (s *authService) Register(ctx context.Context, email, password, name string) (int64, string, string, error) {
	if err := validateRegistration(email, password, name); err != nil {
		return 0, "", "", err
	}

	existing, _ := s.repo.GetByEmail(ctx, email)
	if existing != nil {
		return 0, "", "", ErrUserAlreadyExists
	}

	// Create user in Keycloak
	_, err := s.kc.CreateUser(ctx, email, password, name)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return 0, "", "", ErrUserAlreadyExists
		}
		return 0, "", "", err
	}

	// Save to local DB (without password_hash — Keycloak manages credentials)
	user := &model.User{
		Email:        email,
		Name:         name,
		PasswordHash: "managed-by-keycloak",
	}

	id, err := s.repo.Create(ctx, user)
	if err != nil {
		return 0, "", "", err
	}

	// Get token from Keycloak
	tokenResp, err := s.kc.Login(ctx, email, password)
	if err != nil {
		return 0, "", "", err
	}

	// Publish event to Kafka (async, log errors)
	if s.producer != nil {
		go func() {
			if err := s.producer.PublishUserRegistered(context.Background(), kafkaProducer.UserRegisteredEvent{
				UserID: id,
				Email:  email,
				Name:   name,
			}); err != nil {
				log.Printf("ERROR: failed to publish user.registered event for user_id=%d: %v", id, err)
			}
		}()
	}

	return id, tokenResp.AccessToken, tokenResp.RefreshToken, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (int64, string, string, error) {
	if email == "" || password == "" {
		return 0, "", "", fmt.Errorf("%w: email and password are required", ErrInvalidInput)
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		return 0, "", "", ErrInvalidCredentials
	}

	// Authenticate via Keycloak
	tokenResp, err := s.kc.Login(ctx, email, password)
	if err != nil {
		return 0, "", "", ErrInvalidCredentials
	}

	return user.ID, tokenResp.AccessToken, tokenResp.RefreshToken, nil
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	if refreshToken == "" {
		return "", "", fmt.Errorf("%w: refresh_token is required", ErrInvalidInput)
	}

	tokenResp, err := s.kc.RefreshToken(ctx, refreshToken)
	if err != nil {
		return "", "", ErrInvalidToken
	}

	return tokenResp.AccessToken, tokenResp.RefreshToken, nil
}

func (s *authService) ValidateToken(ctx context.Context, token string) (int64, string, error) {
	_, email, err := s.kc.ValidateToken(ctx, token)
	if err != nil {
		return 0, "", ErrInvalidToken
	}

	// Look up user by email to get the local user ID
	user, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		// User exists in Keycloak but not in local DB
		return 0, email, nil
	}

	return user.ID, user.Email, nil
}

func (s *authService) ChangePassword(ctx context.Context, email, newPassword string) error {
	if email == "" || newPassword == "" {
		return fmt.Errorf("%w: email and new_password are required", ErrInvalidInput)
	}
	if len(newPassword) < minPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLength)
	}
	return s.kc.ResetPassword(ctx, email, newPassword)
}

func validateRegistration(email, password, name string) error {
	if email == "" || password == "" || name == "" {
		return fmt.Errorf("%w: email, password and name are required", ErrInvalidInput)
	}

	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%w: invalid email format", ErrInvalidInput)
	}

	if len(password) < minPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalidInput, minPasswordLength)
	}

	if len(name) < 2 {
		return fmt.Errorf("%w: name must be at least 2 characters", ErrInvalidInput)
	}

	return nil
}
