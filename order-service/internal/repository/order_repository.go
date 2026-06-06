package repository

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/zhanuzak/microservices/order-service/internal/model"
)

type OrderRepository interface {
	Create(ctx context.Context, o *model.Order) (int64, error)
	GetByID(ctx context.Context, id int64) (*model.Order, error)
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Order, error)
	UpdateStatus(ctx context.Context, id int64, status model.OrderStatus) error
}

type orderRepository struct {
	db *sqlx.DB
}

func NewOrderRepository(db *sqlx.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(ctx context.Context, o *model.Order) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO orders (user_id, product_id, quantity, total_price, status)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		o.UserID, o.ProductID, o.Quantity, o.TotalPrice, o.Status,
	).Scan(&id)
	return id, err
}

func (r *orderRepository) GetByID(ctx context.Context, id int64) (*model.Order, error) {
	var o model.Order
	err := r.db.GetContext(ctx, &o, `SELECT * FROM orders WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *orderRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.SelectContext(ctx, &orders,
		`SELECT * FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	return orders, err
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id int64, status model.OrderStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}
