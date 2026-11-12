// Package ports define las dependencias externas del servicio de envíos.
package ports

import (
	"context"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/shipping-service/internal/domain"
)

// ShipmentRepository abstrae la persistencia de envíos.
type ShipmentRepository interface {
	Save(ctx context.Context, shipment *domain.Shipment) error
}

// EventPublisher abstrae la publicación de eventos.
type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, event events.Event) error
}
