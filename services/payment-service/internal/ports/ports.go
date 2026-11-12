// Package ports define las dependencias externas del servicio de pagos.
package ports

import (
	"context"
	"errors"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/payment-service/internal/domain"
)

// ErrNotFound se devuelve cuando no existe el pago de un pedido.
var ErrNotFound = errors.New("pago no encontrado")

// PaymentRepository abstrae la persistencia de pagos.
type PaymentRepository interface {
	Save(ctx context.Context, payment *domain.Payment) error
	Get(ctx context.Context, orderID string) (*domain.Payment, error)
}

// EventPublisher abstrae la publicación de eventos.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, event events.Event) error
}
