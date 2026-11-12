package application

import (
	"context"
	"errors"
	"log/slog"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/order-service/internal/domain"
	"saga-choreography-go/services/order-service/internal/ports"
)

// EventHandler actualiza el estado del pedido a partir de los eventos.
type EventHandler struct {
	repository ports.OrderRepository
	metrics    ports.Metrics
	log        *slog.Logger
}

// NewEventHandler construye el manejador de eventos.
func NewEventHandler(repository ports.OrderRepository, metrics ports.Metrics, log *slog.Logger) *EventHandler {
	return &EventHandler{repository: repository, metrics: metrics, log: log}
}

// OnPaymentCompleted marca el pedido como pagado.
func (h *EventHandler) OnPaymentCompleted(ctx context.Context, event events.Event) error {
	_, err := h.apply(ctx, event, "payment.completed", func(order *domain.Order) error {
		return order.MarkPaymentCompleted()
	})
	return err
}

// OnInventoryReserved marca el pedido con inventario reservado.
func (h *EventHandler) OnInventoryReserved(ctx context.Context, event events.Event) error {
	_, err := h.apply(ctx, event, "inventory.reserved", func(order *domain.Order) error {
		return order.MarkInventoryReserved()
	})
	return err
}

// OnShippingScheduled cierra la saga como COMPLETED.
func (h *EventHandler) OnShippingScheduled(ctx context.Context, event events.Event) error {
	var payload ShippingScheduledPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}
	changed, err := h.apply(ctx, event, "shipping.scheduled", func(order *domain.Order) error {
		order.SetTracking(payload.Tracking)
		return order.MarkCompleted()
	})
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if h.log != nil {
		h.log.Info("saga completada", "saga_id", event.Saga(), "order_id", event.OrderID, "tracking", payload.Tracking)
	}
	h.metrics.SagaCompleted()
	return nil
}

// OnPaymentFailed cancela el pedido porque el cobro fue rechazado.
func (h *EventHandler) OnPaymentFailed(ctx context.Context, event events.Event) error {
	var payload PaymentFailedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}
	return h.cancel(ctx, event, payload.Reason, "payment.failed")
}

// OnPaymentRefunded cancela el pedido después de una compensación.
func (h *EventHandler) OnPaymentRefunded(ctx context.Context, event events.Event) error {
	var payload PaymentRefundedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}
	return h.cancel(ctx, event, payload.Reason, "payment.refunded")
}

// OnInventoryFailed marca el pedido en compensación.
func (h *EventHandler) OnInventoryFailed(ctx context.Context, event events.Event) error {
	var payload InventoryFailedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}
	_, err := h.apply(ctx, event, "inventory.failed", func(order *domain.Order) error {
		return order.MarkCompensating(payload.Reason)
	})
	return err
}

// OnShippingFailed marca el pedido en compensación.
func (h *EventHandler) OnShippingFailed(ctx context.Context, event events.Event) error {
	var payload ShippingFailedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}
	_, err := h.apply(ctx, event, "shipping.failed", func(order *domain.Order) error {
		return order.MarkCompensating(payload.Reason)
	})
	return err
}

func (h *EventHandler) cancel(ctx context.Context, event events.Event, reason, action string) error {
	order, err := h.repository.Get(ctx, event.OrderID)
	if err != nil {
		return err
	}
	if order.Status == domain.StatusCancelled {
		h.logf("evento duplicado o tardío ignorado", event, action)
		return nil
	}
	if err := order.Cancel(reason); err != nil {
		if errors.Is(err, domain.ErrInvalidTransition) {
			h.logf("transición ignorada", event, action)
			return nil
		}
		return err
	}
	if err := h.repository.Update(ctx, order); err != nil {
		return err
	}
	if h.log != nil {
		h.log.Info("acción ejecutada",
			"saga_id", event.Saga(),
			"order_id", event.OrderID,
			"action", action,
			"estado", order.Status,
			"motivo", reason,
		)
	}
	h.metrics.SagaCompensated()
	return nil
}

func (h *EventHandler) apply(ctx context.Context, event events.Event, action string, mutate func(*domain.Order) error) (bool, error) {
	order, err := h.repository.Get(ctx, event.OrderID)
	if err != nil {
		return false, err
	}
	previous := order.Status
	if err := mutate(order); err != nil {
		if errors.Is(err, domain.ErrInvalidTransition) {
			h.logf("transición ignorada", event, action)
			return false, nil
		}
		return false, err
	}
	if err := h.repository.Update(ctx, order); err != nil {
		return false, err
	}
	if h.log != nil {
		h.log.Info("acción ejecutada",
			"saga_id", event.Saga(),
			"order_id", event.OrderID,
			"action", action,
			"estado", order.Status,
		)
	}
	return previous != order.Status, nil
}

func (h *EventHandler) logf(message string, event events.Event, action string) {
	if h.log != nil {
		h.log.Warn(message, "saga_id", event.Saga(), "order_id", event.OrderID, "action", action)
	}
}
