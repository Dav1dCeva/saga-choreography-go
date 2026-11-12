package persistence

import (
	"context"
	"database/sql"

	"saga-choreography-go/pkg/postgres"
)

// Migrate crea el esquema local del servicio de pagos.
func Migrate(ctx context.Context, db *sql.DB) error {
	return postgres.Migrate(ctx, db, []string{
		`CREATE TABLE IF NOT EXISTS payments (
			order_id   TEXT PRIMARY KEY,
			saga_id    TEXT NOT NULL,
			amount     DOUBLE PRECISION NOT NULL,
			status     TEXT NOT NULL,
			reason     TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL
		)`,
	})
}
