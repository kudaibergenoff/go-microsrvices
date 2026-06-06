package model

import "time"

type OrderStatus string

const (
	StatusPending   OrderStatus = "pending"
	StatusConfirmed OrderStatus = "confirmed"
	StatusCancelled OrderStatus = "cancelled"
	StatusCompleted OrderStatus = "completed"
)

type Order struct {
	ID         int64       `json:"id" db:"id"`
	UserID     int64       `json:"user_id" db:"user_id"`
	ProductID  int64       `json:"product_id" db:"product_id"`
	Quantity   int         `json:"quantity" db:"quantity"`
	TotalPrice float64     `json:"total_price" db:"total_price"`
	Status     OrderStatus `json:"status" db:"status"`
	CreatedAt  time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at" db:"updated_at"`
}

type OrderEvent struct {
	OrderID    int64       `json:"order_id"`
	UserID     int64       `json:"user_id"`
	ProductID  int64       `json:"product_id"`
	Quantity   int         `json:"quantity"`
	TotalPrice float64     `json:"total_price"`
	Status     OrderStatus `json:"status"`
}
