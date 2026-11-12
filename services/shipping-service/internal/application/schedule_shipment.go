// Package application implementa los casos de uso del servicio de envíos.
package application

import (
	"context"
	"fmt"
	"log/slog"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/shipping-service/internal/domain"
	"saga-choreography-go/services/shipping-service/internal/ports"
)

// ScheduleShipment consume inventory.reserved y programa el envío.
type ScheduleShipment struct {
	repository ports.ShipmentRepository
	publisher  ports.EventPublisher
	fail       bool
	log        *slog.Logger
}

// NewScheduleShipment construye el caso de uso.
func NewScheduleShipment(repository ports.ShipmentRepository, publisher ports.EventPublisher, fail bool, log *slog.Logger) *ScheduleShipment {
	return &ScheduleShipment{repository: repository, publisher: publisher, fail: fail, log: log}
}

// Handle programa el envío o publica shipping.failed.
func (uc *ScheduleShipment) Handle(ctx context.Context, event events.Event) error {
	var payload InventoryReservedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}

	shipment, err := domain.NewShipment(event.OrderID, event.Saga(), payload.Address)
	if err != nil {
		return err
	}

	reason := ""
	switch {
	case uc.fail:
		reason = "fallo inyectado por FAIL_SHIPPING=true"
	case !domain.ValidAddress(payload.Address):
		reason = "dirección inválida"
	}
	if reason != "" {
		if err := shipment.Fail(reason); err != nil {
			return err
		}
		if err := uc.repository.Save(ctx, shipment); err != nil {
			return err
		}
		uc.logAction(event, "envío rechazado", reason)
		failed, err := events.New(events.ShippingFailed, event.Saga(), event.OrderID, ShippingFailedPayload{
			OrderID: event.OrderID,
			Reason:  reason,
		})
		if err != nil {
			return err
		}
		return uc.publisher.Publish(ctx, events.ShippingFailed, failed)
	}

	tracking := fmt.Sprintf("TRK-%s", event.OrderID)
	if err := shipment.Schedule(tracking); err != nil {
		return err
	}
	if err := uc.repository.Save(ctx, shipment); err != nil {
		return err
	}
	uc.logAction(event, "envío programado", tracking)

	scheduled, err := events.New(events.ShippingScheduled, event.Saga(), event.OrderID, ShippingScheduledPayload{
		OrderID:  event.OrderID,
		Tracking: tracking,
		Address:  payload.Address,
	})
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, events.ShippingScheduled, scheduled)
}

func (uc *ScheduleShipment) logAction(event events.Event, action, detail string) {
	if uc.log == nil {
		return
	}
	uc.log.Info("acción ejecutada",
		"saga_id", event.Saga(),
		"order_id", event.OrderID,
		"action", action,
		"detalle", detail,
	)
}
