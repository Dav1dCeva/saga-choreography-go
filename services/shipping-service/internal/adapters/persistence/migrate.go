package persistence

import (
	"context"
	"database/sql"

	"saga-choreography-go/pkg/postgres"
)

// Migrate crea el esquema local del servicio de envíos.
func Migrate(ctx context.Context, db *sql.DB) error {
	return postgres.Migrate(ctx, db, []string{
		`CREATE TABLE IF NOT EXISTS shipments (
			order_id   TEXT PRIMARY KEY,
			saga_id    TEXT NOT NULL,
			address    TEXT NOT NULL,
			tracking   TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL,
			reason     TEXT NOT NULL DEFAULT '',
			updated_at TIMESTAMPTZ NOT NULL
		)`,
	})
}
