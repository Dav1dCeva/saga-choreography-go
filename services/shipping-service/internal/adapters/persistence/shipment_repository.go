package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"saga-choreography-go/services/shipping-service/internal/domain"
)

// ShipmentRepository implementa ports.ShipmentRepository con PostgreSQL.
type ShipmentRepository struct {
	db *sql.DB
}

// NewShipmentRepository construye el repositorio.
func NewShipmentRepository(db *sql.DB) *ShipmentRepository {
	return &ShipmentRepository{db: db}
}

// Save inserta o actualiza el envío del pedido.
func (r *ShipmentRepository) Save(ctx context.Context, shipment *domain.Shipment) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO shipments (order_id, saga_id, address, tracking, status, reason, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (order_id) DO UPDATE SET
	saga_id = EXCLUDED.saga_id,
	address = EXCLUDED.address,
	tracking = EXCLUDED.tracking,
	status = EXCLUDED.status,
	reason = EXCLUDED.reason,
	updated_at = EXCLUDED.updated_at`,
		shipment.OrderID, shipment.SagaID, shipment.Address, shipment.Tracking,
		shipment.Status, shipment.Reason, shipment.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("guardar envío: %w", err)
	}
	return nil
}
