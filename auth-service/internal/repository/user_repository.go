package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/zhanuzak/microservices/auth-service/internal/model"
)

type UserRepository interface {
	Create(ctx context.Context, user *model.User) (int64, error)
	GetByEmail(ctx context.Context, email string) (*model.User, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
}

type userRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *model.User) (int64, error) {
	var id int64
	err := r.db.QueryRowxContext(ctx,
		`INSERT INTO users (email, name, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		user.Email, user.Name, user.PasswordHash,
	).Scan(&id)
	return id, err
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	user := &model.User{}
	err := r.db.GetContext(ctx, user,
		`SELECT id, email, name, password_hash, created_at, updated_at FROM users WHERE email = $1`,
		email,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	user := &model.User{}
	err := r.db.GetContext(ctx, user,
		`SELECT id, email, name, password_hash, created_at, updated_at FROM users WHERE id = $1`,
		id,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}
