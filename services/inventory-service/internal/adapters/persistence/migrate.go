package persistence

import (
	"context"
	"database/sql"

	"saga-choreography-go/pkg/postgres"
)

// Migrate crea el esquema local del servicio de inventario.
func Migrate(ctx context.Context, db *sql.DB) error {
	return postgres.Migrate(ctx, db, []string{
		`CREATE TABLE IF NOT EXISTS stock (
			sku       TEXT PRIMARY KEY,
			available INTEGER NOT NULL CHECK (available >= 0)
		)`,
		`CREATE TABLE IF NOT EXISTS reservations (
			order_id   TEXT PRIMARY KEY,
			saga_id    TEXT NOT NULL,
			sku        TEXT NOT NULL,
			quantity   INTEGER NOT NULL CHECK (quantity > 0),
			status     TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		)`,
		`INSERT INTO stock (sku, available) VALUES
			('laptop', 3),
			('phone', 10),
			('headphones', 20),
			('monitor', 4)
		ON CONFLICT (sku) DO NOTHING`,
	})
}
