package persistence

import (
	"context"
	"database/sql"

	"saga-choreography-go/pkg/postgres"
)

// Migrate crea el esquema local del servicio de pedidos.
func Migrate(ctx context.Context, db *sql.DB) error {
	return postgres.Migrate(ctx, db, []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id          TEXT PRIMARY KEY,
			saga_id     TEXT NOT NULL,
			customer    TEXT NOT NULL,
			product     TEXT NOT NULL,
			quantity    INTEGER NOT NULL,
			unit_price  DOUBLE PRECISION NOT NULL,
			amount      DOUBLE PRECISION NOT NULL,
			address     TEXT NOT NULL,
			tracking    TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL,
			reason      TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL,
			updated_at  TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders (created_at DESC)`,
	})
}
