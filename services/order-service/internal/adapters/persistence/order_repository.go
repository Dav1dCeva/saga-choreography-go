package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"saga-choreography-go/services/order-service/internal/domain"
	"saga-choreography-go/services/order-service/internal/ports"
)

// OrderRepository implementa ports.OrderRepository con PostgreSQL.
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository construye el repositorio.
func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create inserta un pedido nuevo.
func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO orders (id, saga_id, customer, product, quantity, unit_price, amount, address,
                    tracking, status, reason, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		order.ID, order.SagaID, order.Customer, order.Product, order.Quantity,
		order.UnitPrice, order.Amount, order.Address, order.Tracking, order.Status,
		order.Reason, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insertar pedido: %w", err)
	}
	return nil
}

// Update guarda el estado actual del pedido.
func (r *OrderRepository) Update(ctx context.Context, order *domain.Order) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE orders
SET status = $2, reason = $3, tracking = $4, updated_at = $5
WHERE id = $1`,
		order.ID, order.Status, order.Reason, order.Tracking, order.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("actualizar pedido: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ports.ErrNotFound
	}
	return nil
}

// Get busca un pedido por su identificador.
func (r *OrderRepository) Get(ctx context.Context, id string) (*domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT id, saga_id, customer, product, quantity, unit_price, amount, address,
       tracking, status, reason, created_at, updated_at
FROM orders WHERE id = $1`, id)
	order, err := scanOrder(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar pedido: %w", err)
	}
	return order, nil
}

// List devuelve los pedidos más recientes primero.
func (r *OrderRepository) List(ctx context.Context) ([]domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, saga_id, customer, product, quantity, unit_price, amount, address,
       tracking, status, reason, created_at, updated_at
FROM orders ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listar pedidos: %w", err)
	}
	defer func() { _ = rows.Close() }()

	orders := make([]domain.Order, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return orders, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanOrder(row scanner) (*domain.Order, error) {
	var order domain.Order
	err := row.Scan(
		&order.ID, &order.SagaID, &order.Customer, &order.Product, &order.Quantity,
		&order.UnitPrice, &order.Amount, &order.Address, &order.Tracking, &order.Status,
		&order.Reason, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &order, nil
}
