package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"saga-choreography-go/services/inventory-service/internal/domain"
	"saga-choreography-go/services/inventory-service/internal/ports"
)

// InventoryRepository implementa ports.InventoryRepository con PostgreSQL.
type InventoryRepository struct {
	db *sql.DB
}

// NewInventoryRepository construye el repositorio.
func NewInventoryRepository(db *sql.DB) *InventoryRepository {
	return &InventoryRepository{db: db}
}

// Reserve descuenta stock e inserta la reserva en una transacción.
func (r *InventoryRepository) Reserve(ctx context.Context, reservation *domain.Reservation) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("iniciar transacción de reserva: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var existing string
	err = tx.QueryRowContext(ctx, `SELECT order_id FROM reservations WHERE order_id = $1`, reservation.OrderID).Scan(&existing)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("buscar reserva existente: %w", err)
	}

	var available int
	err = tx.QueryRowContext(ctx, `SELECT available FROM stock WHERE sku = $1 FOR UPDATE`, reservation.SKU).Scan(&available)
	if errors.Is(err, sql.ErrNoRows) {
		return false, domain.ErrInsufficientStock
	}
	if err != nil {
		return false, fmt.Errorf("leer stock: %w", err)
	}
	if available < reservation.Quantity {
		return false, domain.ErrInsufficientStock
	}

	if _, err := tx.ExecContext(ctx, `UPDATE stock SET available = available - $1 WHERE sku = $2`, reservation.Quantity, reservation.SKU); err != nil {
		return false, fmt.Errorf("descontar stock: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO reservations (order_id, saga_id, sku, quantity, status, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)`,
		reservation.OrderID, reservation.SagaID, reservation.SKU, reservation.Quantity, reservation.Status, reservation.UpdatedAt,
	); err != nil {
		return false, fmt.Errorf("insertar reserva: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("confirmar reserva: %w", err)
	}
	return true, nil
}

// Release libera stock de una reserva activa dentro de una transacción.
func (r *InventoryRepository) Release(ctx context.Context, orderID string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("iniciar transacción de liberación: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		sku      string
		quantity int
		status   domain.Status
	)
	err = tx.QueryRowContext(ctx, `
SELECT sku, quantity, status FROM reservations WHERE order_id = $1 FOR UPDATE`, orderID).
		Scan(&sku, &quantity, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ports.ErrNotFound
	}
	if err != nil {
		return false, fmt.Errorf("buscar reserva: %w", err)
	}
	if status != domain.StatusReserved {
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE reservations SET status = $2, updated_at = now() WHERE order_id = $1`,
		orderID, domain.StatusReleased,
	); err != nil {
		return false, fmt.Errorf("marcar reserva liberada: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE stock SET available = available + $1 WHERE sku = $2`, quantity, sku); err != nil {
		return false, fmt.Errorf("reponer stock: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("confirmar liberación: %w", err)
	}
	return true, nil
}

// Available devuelve las unidades disponibles de un SKU.
func (r *InventoryRepository) Available(ctx context.Context, sku string) (int, error) {
	var available int
	err := r.db.QueryRowContext(ctx, `SELECT available FROM stock WHERE sku = $1`, sku).Scan(&available)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("consultar stock: %w", err)
	}
	return available, nil
}
