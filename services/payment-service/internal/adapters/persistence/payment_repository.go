package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"saga-choreography-go/services/payment-service/internal/domain"
	"saga-choreography-go/services/payment-service/internal/ports"
)

// PaymentRepository implementa ports.PaymentRepository con PostgreSQL.
type PaymentRepository struct {
	db *sql.DB
}

// NewPaymentRepository construye el repositorio.
func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

// Save inserta o actualiza el pago del pedido.
func (r *PaymentRepository) Save(ctx context.Context, payment *domain.Payment) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO payments (order_id, saga_id, amount, status, reason, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (order_id) DO UPDATE SET
	saga_id = EXCLUDED.saga_id,
	amount = EXCLUDED.amount,
	status = EXCLUDED.status,
	reason = EXCLUDED.reason,
	updated_at = EXCLUDED.updated_at`,
		payment.OrderID, payment.SagaID, payment.Amount, payment.Status, payment.Reason, payment.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("guardar pago: %w", err)
	}
	return nil
}

// Get busca el pago por order_id.
func (r *PaymentRepository) Get(ctx context.Context, orderID string) (*domain.Payment, error) {
	var payment domain.Payment
	err := r.db.QueryRowContext(ctx, `
SELECT order_id, saga_id, amount, status, reason, updated_at
FROM payments WHERE order_id = $1`, orderID).
		Scan(&payment.OrderID, &payment.SagaID, &payment.Amount, &payment.Status, &payment.Reason, &payment.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ports.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("buscar pago: %w", err)
	}
	return &payment, nil
}
