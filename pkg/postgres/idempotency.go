package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

// IdempotencyStore guarda los event_id procesados por un servicio.
type IdempotencyStore struct {
	db *sql.DB
}

// NewIdempotencyStore crea el almacén de idempotencia.
func NewIdempotencyStore(db *sql.DB) *IdempotencyStore {
	return &IdempotencyStore{db: db}
}

// EnsureTable crea la tabla processed_events si no existe.
func (s *IdempotencyStore) EnsureTable(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS processed_events (
	event_id     TEXT PRIMARY KEY,
	event_type   TEXT NOT NULL,
	processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);`
	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("crear processed_events: %w", err)
	}
	return nil
}

// MarkProcessed devuelve true solo la primera vez que se ve un event_id.
func (s *IdempotencyStore) MarkProcessed(ctx context.Context, eventID, eventType string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `
INSERT INTO processed_events (event_id, event_type, processed_at)
VALUES ($1, $2, now())
ON CONFLICT (event_id) DO NOTHING`, eventID, eventType)
	if err != nil {
		return false, fmt.Errorf("marcar evento %s: %w", eventID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows == 1, nil
}

// UnmarkProcessed elimina el registro para permitir un reintento limpio.
func (s *IdempotencyStore) UnmarkProcessed(ctx context.Context, eventID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM processed_events WHERE event_id = $1`, eventID); err != nil {
		return fmt.Errorf("desmarcar evento %s: %w", eventID, err)
	}
	return nil
}
