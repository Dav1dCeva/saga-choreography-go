// Package domain contiene el agregado Pago y sus reglas de negocio.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status representa el estado local del pago.
type Status string

const (
	StatusPending    Status = "PENDING"
	StatusAuthorized Status = "AUTHORIZED"
	StatusFailed     Status = "FAILED"
	StatusRefunded   Status = "REFUNDED"
)

var (
	// ErrInvalidPayment indica datos de pago inválidos.
	ErrInvalidPayment = errors.New("pago inválido")
	// ErrInvalidTransition indica un cambio de estado no permitido.
	ErrInvalidTransition = errors.New("transición de pago inválida")
)

// Payment es el registro local del cobro de un pedido.
type Payment struct {
	OrderID   string
	SagaID    string
	Amount    float64
	Status    Status
	Reason    string
	UpdatedAt time.Time
}

// NewPayment crea un pago en estado PENDING.
func NewPayment(orderID, sagaID string, amount float64) (*Payment, error) {
	orderID = strings.TrimSpace(orderID)
	sagaID = strings.TrimSpace(sagaID)
	switch {
	case orderID == "":
		return nil, fmt.Errorf("%w: order_id vacío", ErrInvalidPayment)
	case sagaID == "":
		return nil, fmt.Errorf("%w: saga_id vacío", ErrInvalidPayment)
	case amount < 0:
		return nil, fmt.Errorf("%w: monto negativo", ErrInvalidPayment)
	}
	return &Payment{
		OrderID:   orderID,
		SagaID:    sagaID,
		Amount:    amount,
		Status:    StatusPending,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// Authorize confirma el cobro.
func (p *Payment) Authorize() error {
	switch p.Status {
	case StatusAuthorized:
		return nil
	case StatusPending:
		p.Status = StatusAuthorized
		p.Reason = ""
		p.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return p.transitionError(StatusAuthorized)
	}
}

// Fail rechaza el cobro con un motivo.
func (p *Payment) Fail(reason string) error {
	switch p.Status {
	case StatusFailed:
		return nil
	case StatusPending:
		p.Status = StatusFailed
		p.Reason = reason
		p.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return p.transitionError(StatusFailed)
	}
}

// Refund reembolsa un pago previamente autorizado.
func (p *Payment) Refund(reason string) error {
	switch p.Status {
	case StatusRefunded:
		return nil
	case StatusAuthorized:
		p.Status = StatusRefunded
		p.Reason = reason
		p.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return p.transitionError(StatusRefunded)
	}
}

// IsAuthorized indica si el pago puede reembolsarse.
func (p *Payment) IsAuthorized() bool {
	return p.Status == StatusAuthorized
}

func (p *Payment) transitionError(target Status) error {
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, p.Status, target)
}
