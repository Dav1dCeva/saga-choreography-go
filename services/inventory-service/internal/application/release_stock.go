package application

import (
	"context"
	"errors"
	"log/slog"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/inventory-service/internal/ports"
)

// ReleaseStock consume shipping.failed y libera el stock reservado.
type ReleaseStock struct {
	repository ports.InventoryRepository
	publisher  ports.EventPublisher
	log        *slog.Logger
}

// NewReleaseStock construye el caso de uso de compensación.
func NewReleaseStock(repository ports.InventoryRepository, publisher ports.EventPublisher, log *slog.Logger) *ReleaseStock {
	return &ReleaseStock{repository: repository, publisher: publisher, log: log}
}

// Handle libera la reserva y publica inventory.released.
func (uc *ReleaseStock) Handle(ctx context.Context, event events.Event) error {
	var payload ShippingFailedPayload
	_ = event.Decode(&payload)

	released, err := uc.repository.Release(ctx, event.OrderID)
	if errors.Is(err, ports.ErrNotFound) {
		if uc.log != nil {
			uc.log.Warn("no hay reserva que liberar", "order_id", event.OrderID, "event_id", event.EventID)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !released {
		if uc.log != nil {
			uc.log.Info("reserva ya liberada; evento ignorado", "order_id", event.OrderID)
		}
		return nil
	}

	reason := payload.Reason
	if reason == "" {
		reason = "compensación por envío fallido"
	}
	if uc.log != nil {
		uc.log.Info("acción ejecutada",
			"saga_id", event.Saga(),
			"order_id", event.OrderID,
			"action", "stock liberado",
			"motivo", reason,
		)
	}

	releasedEvent, err := events.New(events.InventoryReleased, event.Saga(), event.OrderID, InventoryReleasedPayload{
		OrderID: event.OrderID,
		Reason:  reason,
	})
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, events.InventoryReleased, releasedEvent)
}
