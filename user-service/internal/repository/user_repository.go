package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/zhanuzak/microservices/user-service/internal/model"
)

var (
	ErrNotFound      = errors.New("record not found")
	ErrDuplicateEmail = errors.New("email already exists")
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) error
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	UpdateAvatarURL(ctx context.Context, id int64, avatarURL string) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, page, limit int) ([]*model.User, int, error)
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`,
		user.ID, user.Email, user.Name,
	)
	return err
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	user := &model.User{}
	err := r.db.GetContext(ctx, user,
		`SELECT id, email, name, avatar_url, created_at, updated_at FROM users WHERE id = $1`,
		id,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := r.db.GetContext(ctx, user,
		`SELECT id, email, name, avatar_url, created_at, updated_at FROM users WHERE email = $1`,
		email,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (r *userRepository) Update(ctx context.Context, user *model.User) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET email = $1, name = $2, updated_at = NOW() WHERE id = $3`,
		user.Email, user.Name, user.ID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update user rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *userRepository) UpdateAvatarURL(ctx context.Context, id int64, avatarURL string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE users SET avatar_url = $1, updated_at = NOW() WHERE id = $2`,
		avatarURL, id,
	)
	if err != nil {
		return fmt.Errorf("update avatar_url: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update avatar_url rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM users WHERE id = $1`, id,
	)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *userRepository) List(ctx context.Context, page, limit int) ([]*model.User, int, error) {
	var total int
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM users`)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	offset := (page - 1) * limit
	var users []*model.User
	err = r.db.SelectContext(ctx, &users,
		`SELECT id, email, name, avatar_url, created_at, updated_at FROM users ORDER BY id LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	return users, total, nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
