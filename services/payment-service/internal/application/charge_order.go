// Package application implementa los casos de uso del servicio de pagos.
package application

import (
	"context"
	"fmt"
	"log/slog"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/payment-service/internal/domain"
	"saga-choreography-go/services/payment-service/internal/ports"
)

// ChargeConfig contiene las reglas de negocio y la inyección de fallos.
type ChargeConfig struct {
	FailPayment     bool
	MaxAmount       float64
	BlockedCustomer string
}

// ChargeOrder consume order.created, cobra y publica payment.completed o
// payment.failed.
type ChargeOrder struct {
	repository ports.PaymentRepository
	publisher  ports.EventPublisher
	config     ChargeConfig
	log        *slog.Logger
}

// NewChargeOrder construye el caso de uso.
func NewChargeOrder(repository ports.PaymentRepository, publisher ports.EventPublisher, config ChargeConfig, log *slog.Logger) *ChargeOrder {
	return &ChargeOrder{repository: repository, publisher: publisher, config: config, log: log}
}

// Handle ejecuta el cobro del pedido.
func (uc *ChargeOrder) Handle(ctx context.Context, event events.Event) error {
	var payload OrderCreatedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}

	payment, err := domain.NewPayment(event.OrderID, event.Saga(), payload.Amount)
	if err != nil {
		return err
	}

	reason := uc.failureReason(payload)
	if reason != "" {
		if err := payment.Fail(reason); err != nil {
			return err
		}
		if err := uc.repository.Save(ctx, payment); err != nil {
			return err
		}
		uc.logAction(event, "pago rechazado", reason)
		return uc.publish(ctx, event, events.PaymentFailed, PaymentFailedPayload{
			OrderID: payment.OrderID,
			Reason:  reason,
			Amount:  payment.Amount,
		})
	}

	if err := payment.Authorize(); err != nil {
		return err
	}
	if err := uc.repository.Save(ctx, payment); err != nil {
		return err
	}
	uc.logAction(event, "pago autorizado", "")
	return uc.publish(ctx, event, events.PaymentCompleted, PaymentCompletedPayload{
		OrderID:  payment.OrderID,
		Product:  payload.Product,
		Quantity: payload.Quantity,
		Amount:   payment.Amount,
		Address:  payload.Address,
	})
}

func (uc *ChargeOrder) failureReason(payload OrderCreatedPayload) string {
	switch {
	case uc.config.FailPayment:
		return "fallo inyectado por FAIL_PAYMENT=true"
	case payload.Amount <= 0:
		return "monto inválido"
	case uc.config.MaxAmount > 0 && payload.Amount > uc.config.MaxAmount:
		return fmt.Sprintf("el monto supera el límite de %.2f", uc.config.MaxAmount)
	case uc.config.BlockedCustomer != "" && payload.Customer == uc.config.BlockedCustomer:
		return "cliente bloqueado por antifraude"
	default:
		return ""
	}
}

func (uc *ChargeOrder) publish(ctx context.Context, source events.Event, routingKey string, payload any) error {
	event, err := events.New(routingKey, source.Saga(), source.OrderID, payload)
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, routingKey, event)
}

func (uc *ChargeOrder) logAction(event events.Event, action, reason string) {
	if uc.log == nil {
		return
	}
	args := []any{"saga_id", event.Saga(), "order_id", event.OrderID, "action", action}
	if reason != "" {
		args = append(args, "motivo", reason)
	}
	uc.log.Info("acción ejecutada", args...)
}
