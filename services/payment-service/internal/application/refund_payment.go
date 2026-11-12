package application

import (
	"context"
	"errors"
	"log/slog"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/payment-service/internal/ports"
)

// RefundPayment ejecuta la compensación del pago ante un fallo posterior.
type RefundPayment struct {
	repository ports.PaymentRepository
	publisher  ports.EventPublisher
	log        *slog.Logger
}

// NewRefundPayment construye el caso de uso de reembolso.
func NewRefundPayment(repository ports.PaymentRepository, publisher ports.EventPublisher, log *slog.Logger) *RefundPayment {
	return &RefundPayment{repository: repository, publisher: publisher, log: log}
}

// Handle reembolsa si el pago estaba autorizado y publica payment.refunded.
func (uc *RefundPayment) Handle(ctx context.Context, event events.Event, defaultReason string) error {
	payment, err := uc.repository.Get(ctx, event.OrderID)
	if errors.Is(err, ports.ErrNotFound) {
		if uc.log != nil {
			uc.log.Warn("no hay pago que reembolsar", "order_id", event.OrderID, "event_id", event.EventID)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if !payment.IsAuthorized() {
		if uc.log != nil {
			uc.log.Info("reembolso omitido", "order_id", event.OrderID, "estado", payment.Status)
		}
		return nil
	}

	reason := defaultReason
	var payload struct {
		Reason string `json:"reason"`
	}
	if err := event.Decode(&payload); err == nil && payload.Reason != "" {
		reason = payload.Reason
	}

	if err := payment.Refund(reason); err != nil {
		return err
	}
	if err := uc.repository.Save(ctx, payment); err != nil {
		return err
	}
	if uc.log != nil {
		uc.log.Info("acción ejecutada",
			"saga_id", event.Saga(),
			"order_id", event.OrderID,
			"action", "pago reembolsado",
			"motivo", reason,
		)
	}

	refunded, err := events.New(events.PaymentRefunded, event.Saga(), event.OrderID, PaymentRefundedPayload{
		OrderID: payment.OrderID,
		Reason:  reason,
		Amount:  payment.Amount,
	})
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, events.PaymentRefunded, refunded)
}
