// Package domain contiene la reserva de stock y sus reglas de negocio.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status representa el estado de una reserva de inventario.
type Status string

const (
	StatusReserved Status = "RESERVED"
	StatusReleased Status = "RELEASED"
)

var (
	// ErrInvalidReservation indica datos de reserva inválidos.
	ErrInvalidReservation = errors.New("reserva inválida")
	// ErrInsufficientStock indica que no hay unidades suficientes.
	ErrInsufficientStock = errors.New("stock insuficiente")
	// ErrInvalidTransition indica un cambio de estado no permitido.
	ErrInvalidTransition = errors.New("transición de reserva inválida")
)

// Reservation es la reserva local de un SKU para un pedido.
type Reservation struct {
	OrderID   string
	SagaID    string
	SKU       string
	Quantity  int
	Status    Status
	UpdatedAt time.Time
}

// NewReservation crea una reserva en estado RESERVED.
func NewReservation(orderID, sagaID, sku string, quantity int) (*Reservation, error) {
	orderID = strings.TrimSpace(orderID)
	sagaID = strings.TrimSpace(sagaID)
	sku = strings.TrimSpace(sku)
	switch {
	case orderID == "":
		return nil, fmt.Errorf("%w: order_id vacío", ErrInvalidReservation)
	case sagaID == "":
		return nil, fmt.Errorf("%w: saga_id vacío", ErrInvalidReservation)
	case sku == "":
		return nil, fmt.Errorf("%w: sku vacío", ErrInvalidReservation)
	case quantity <= 0:
		return nil, fmt.Errorf("%w: cantidad debe ser mayor que cero", ErrInvalidReservation)
	}
	return &Reservation{
		OrderID:   orderID,
		SagaID:    sagaID,
		SKU:       sku,
		Quantity:  quantity,
		Status:    StatusReserved,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// Release libera la reserva de stock.
func (r *Reservation) Release() error {
	switch r.Status {
	case StatusReleased:
		return nil
	case StatusReserved:
		r.Status = StatusReleased
		r.UpdatedAt = time.Now().UTC()
		return nil
	default:
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, r.Status, StatusReleased)
	}
}

// IsReserved indica si la reserva sigue activa.
func (r *Reservation) IsReserved() bool {
	return r.Status == StatusReserved
}
