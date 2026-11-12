// Package domain contiene el agregado Envío y sus reglas de negocio.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status representa el estado del envío.
type Status string

const (
	StatusPending   Status = "PENDING"
	StatusScheduled Status = "SCHEDULED"
	StatusFailed    Status = "FAILED"
)

var (
	// ErrInvalidShipment indica datos de envío inválidos.
	ErrInvalidShipment = errors.New("envío inválido")
	// ErrInvalidTransition indica un cambio de estado no permitido.
	ErrInvalidTransition = errors.New("transición de envío inválida")
)

// Shipment es la programación local del envío de un pedido.
type Shipment struct {
	OrderID   string
	SagaID    string
	Address   string
	Tracking  string
	Status    Status
	Reason    string
	UpdatedAt time.Time
}

// NewShipment crea un envío en estado PENDING.
func NewShipment(orderID, sagaID, address string) (*Shipment, error) {
	orderID = strings.TrimSpace(orderID)
	sagaID = strings.TrimSpace(sagaID)
	switch {
	case orderID == "":
		return nil, fmt.Errorf("%w: order_id vacío", ErrInvalidShipment)
	case sagaID == "":
		return nil, fmt.Errorf("%w: saga_id vacío", ErrInvalidShipment)
	}
	return &Shipment{
		OrderID:   orderID,
		SagaID:    sagaID,
		Address:   strings.TrimSpace(address),
		Status:    StatusPending,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// Schedule programa el envío con un número de seguimiento.
func (s *Shipment) Schedule(tracking string) error {
	switch s.Status {
	case StatusScheduled:
		return nil
	case StatusPending:
		s.Status = StatusScheduled
		s.Tracking = tracking
		s.Reason = ""
		s.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return s.transitionError(StatusScheduled)
	}
}

// Fail marca el envío como fallido.
func (s *Shipment) Fail(reason string) error {
	switch s.Status {
	case StatusFailed:
		return nil
	case StatusPending:
		s.Status = StatusFailed
		s.Reason = reason
		s.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return s.transitionError(StatusFailed)
	}
}

// ValidAddress aplica la regla de negocio de direcciones de la demo.
func ValidAddress(address string) bool {
	address = strings.TrimSpace(address)
	if address == "" {
		return false
	}
	return !strings.Contains(strings.ToLower(address), "invalid")
}

func (s *Shipment) transitionError(target Status) error {
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, s.Status, target)
}
