// Package application implementa los casos de uso del servicio de inventario.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/inventory-service/internal/domain"
	"saga-choreography-go/services/inventory-service/internal/ports"
)

// ReserveStock consume payment.completed y reserva el stock del pedido.
type ReserveStock struct {
	repository ports.InventoryRepository
	publisher  ports.EventPublisher
	fail       bool
	log        *slog.Logger
}

// NewReserveStock construye el caso de uso.
func NewReserveStock(repository ports.InventoryRepository, publisher ports.EventPublisher, fail bool, log *slog.Logger) *ReserveStock {
	return &ReserveStock{repository: repository, publisher: publisher, fail: fail, log: log}
}

// Handle reserva stock y publica inventory.reserved o inventory.failed.
func (uc *ReserveStock) Handle(ctx context.Context, event events.Event) error {
	var payload PaymentCompletedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}

	if uc.fail {
		return uc.failEvent(ctx, event, payload.Product, "fallo inyectado por FAIL_INVENTORY=true")
	}

	reservation, err := domain.NewReservation(event.OrderID, event.Saga(), payload.Product, payload.Quantity)
	if err != nil {
		return uc.failEvent(ctx, event, payload.Product, err.Error())
	}

	created, err := uc.repository.Reserve(ctx, reservation)
	if errors.Is(err, domain.ErrInsufficientStock) {
		available, availableErr := uc.repository.Available(ctx, payload.Product)
		if availableErr != nil {
			available = 0
		}
		return uc.failEvent(ctx, event, payload.Product,
			fmt.Sprintf("stock insuficiente para %s (disponible: %d)", payload.Product, available))
	}
	if err != nil {
		return err
	}
	if !created {
		uc.logInfo("reserva ya existente; evento ignorado", event, payload.Product)
		return nil
	}

	uc.logInfo("stock reservado", event, payload.Product)
	reserved, err := events.New(events.InventoryReserved, event.Saga(), event.OrderID, InventoryReservedPayload{
		OrderID:  event.OrderID,
		Product:  payload.Product,
		Quantity: payload.Quantity,
		Address:  payload.Address,
	})
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, events.InventoryReserved, reserved)
}

func (uc *ReserveStock) failEvent(ctx context.Context, source events.Event, product, reason string) error {
	uc.logInfo("reserva rechazada: "+reason, source, product)
	failed, err := events.New(events.InventoryFailed, source.Saga(), source.OrderID, InventoryFailedPayload{
		OrderID: source.OrderID,
		Product: product,
		Reason:  reason,
	})
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, events.InventoryFailed, failed)
}

func (uc *ReserveStock) logInfo(action string, event events.Event, product string) {
	if uc.log == nil {
		return
	}
	uc.log.Info("acción ejecutada",
		"saga_id", event.Saga(),
		"order_id", event.OrderID,
		"action", action,
		"product", product,
	)
}
