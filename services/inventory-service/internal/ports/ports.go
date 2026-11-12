// Package ports define las dependencias externas del servicio de inventario.
package ports

import (
	"context"
	"errors"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/inventory-service/internal/domain"
)

// ErrNotFound se devuelve cuando no existe una reserva.
var ErrNotFound = errors.New("reserva no encontrada")

// InventoryRepository abstrae el stock y las reservas.
type InventoryRepository interface {
	// Reserve devuelve true si creó la reserva y descontó stock.
	Reserve(ctx context.Context, reservation *domain.Reservation) (bool, error)
	// Release devuelve true si liberó una reserva activa.
	Release(ctx context.Context, orderID string) (bool, error)
	// Available devuelve las unidades disponibles de un SKU.
	Available(ctx context.Context, sku string) (int, error)
}

// EventPublisher abstrae la publicación de eventos.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, event events.Event) error
}
