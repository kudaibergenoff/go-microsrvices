package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/zhanuzak/microservices/product-service/internal/model"
)

type ProductRepository interface {
	Create(ctx context.Context, p *model.Product) (int64, error)
	GetByID(ctx context.Context, id int64) (*model.Product, error)
	List(ctx context.Context, category string, limit, offset int) ([]model.Product, error)
	Update(ctx context.Context, p *model.Product) error
	Delete(ctx context.Context, id int64) error
	Search(ctx context.Context, query string, limit int) ([]model.Product, error)
	ReserveStock(ctx context.Context, productID int64, quantity int) error
	ReleaseStock(ctx context.Context, productID int64, quantity int) error
}

type productRepository struct {
	db *sqlx.DB
}

func NewProductRepository(db *sqlx.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) Create(ctx context.Context, p *model.Product) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO products (name, description, price, category, image_url, stock)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		p.Name, p.Description, p.Price, p.Category, p.ImageURL, p.Stock,
	).Scan(&id)
	return id, err
}

func (r *productRepository) GetByID(ctx context.Context, id int64) (*model.Product, error) {
	var p model.Product
	err := r.db.GetContext(ctx, &p, `SELECT * FROM products WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *productRepository) List(ctx context.Context, category string, limit, offset int) ([]model.Product, error) {
	var products []model.Product
	var err error

	if category != "" {
		err = r.db.SelectContext(ctx, &products,
			`SELECT * FROM products WHERE category = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			category, limit, offset)
	} else {
		err = r.db.SelectContext(ctx, &products,
			`SELECT * FROM products ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			limit, offset)
	}
	return products, err
}

func (r *productRepository) Update(ctx context.Context, p *model.Product) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE products SET name=$1, description=$2, price=$3, category=$4, image_url=$5, stock=$6, updated_at=NOW() WHERE id=$7`,
		p.Name, p.Description, p.Price, p.Category, p.ImageURL, p.Stock, p.ID)
	return err
}

func (r *productRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM products WHERE id = $1`, id)
	return err
}

func (r *productRepository) Search(ctx context.Context, query string, limit int) ([]model.Product, error) {
	var products []model.Product
	err := r.db.SelectContext(ctx, &products,
		`SELECT * FROM products WHERE name ILIKE $1 OR description ILIKE $1 ORDER BY created_at DESC LIMIT $2`,
		"%"+query+"%", limit)
	return products, err
}

func (r *productRepository) ReserveStock(ctx context.Context, productID int64, quantity int) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE products SET stock = stock - $1, updated_at = NOW() WHERE id = $2 AND stock >= $1`,
		quantity, productID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("insufficient stock for product %d", productID)
	}
	return nil
}

func (r *productRepository) ReleaseStock(ctx context.Context, productID int64, quantity int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE products SET stock = stock + $1, updated_at = NOW() WHERE id = $2`,
		quantity, productID)
	return err
}
