// Package ports define las interfaces que la aplicación necesita.
package ports

import (
	"context"
	"errors"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/order-service/internal/domain"
)

// ErrNotFound se devuelve cuando no existe un pedido.
var ErrNotFound = errors.New("pedido no encontrado")

// OrderRepository abstrae la persistencia de pedidos.
type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
	Update(ctx context.Context, order *domain.Order) error
	Get(ctx context.Context, id string) (*domain.Order, error)
	List(ctx context.Context) ([]domain.Order, error)
}

// EventPublisher abstrae la publicación de eventos.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, event events.Event) error
}

// Metrics abstrae los contadores de negocio.
type Metrics interface {
	SagaStarted()
	SagaCompleted()
	SagaCompensated()
}
