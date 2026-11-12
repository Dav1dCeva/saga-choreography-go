// Package domain contiene la entidad Pedido y sus reglas de estado.
// No debe importar adaptadores, RabbitMQ, HTTP ni PostgreSQL.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Status representa el estado local del pedido dentro de la saga.
type Status string

const (
	StatusPending           Status = "PENDING"
	StatusPaymentCompleted  Status = "PAYMENT_COMPLETED"
	StatusInventoryReserved Status = "INVENTORY_RESERVED"
	StatusCompleted         Status = "COMPLETED"
	StatusCompensating      Status = "COMPENSATING"
	StatusCancelled         Status = "CANCELLED"
)

var (
	// ErrInvalidOrder indica datos de pedido incompletos o inválidos.
	ErrInvalidOrder = errors.New("pedido inválido")
	// ErrInvalidTransition indica un cambio de estado no permitido.
	ErrInvalidTransition = errors.New("transición de estado inválida")
)

// Order es la raíz de agregado del servicio de pedidos.
type Order struct {
	ID        string    `json:"id"`
	SagaID    string    `json:"sagaId"`
	Customer  string    `json:"customer"`
	Product   string    `json:"product"`
	Quantity  int       `json:"quantity"`
	UnitPrice float64   `json:"unitPrice"`
	Amount    float64   `json:"amount"`
	Address   string    `json:"address"`
	Tracking  string    `json:"tracking"`
	Status    Status    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NewOrderInput agrupa los datos necesarios para crear un pedido.
type NewOrderInput struct {
	ID        string
	SagaID    string
	Customer  string
	Product   string
	Quantity  int
	UnitPrice float64
	Address   string
}

// NewOrder valida y construye un pedido en estado PENDING.
func NewOrder(input NewOrderInput) (*Order, error) {
	input.ID = strings.TrimSpace(input.ID)
	input.SagaID = strings.TrimSpace(input.SagaID)
	input.Customer = strings.TrimSpace(input.Customer)
	input.Product = strings.TrimSpace(input.Product)
	input.Address = strings.TrimSpace(input.Address)

	switch {
	case input.ID == "":
		return nil, fmt.Errorf("%w: id vacío", ErrInvalidOrder)
	case input.SagaID == "":
		return nil, fmt.Errorf("%w: saga_id vacío", ErrInvalidOrder)
	case input.Customer == "":
		return nil, fmt.Errorf("%w: cliente vacío", ErrInvalidOrder)
	case input.Product == "":
		return nil, fmt.Errorf("%w: producto vacío", ErrInvalidOrder)
	case input.Quantity <= 0:
		return nil, fmt.Errorf("%w: cantidad debe ser mayor que cero", ErrInvalidOrder)
	case input.UnitPrice <= 0:
		return nil, fmt.Errorf("%w: precio unitario debe ser mayor que cero", ErrInvalidOrder)
	case input.Address == "":
		return nil, fmt.Errorf("%w: dirección vacía", ErrInvalidOrder)
	}

	now := time.Now().UTC()
	return &Order{
		ID:        input.ID,
		SagaID:    input.SagaID,
		Customer:  input.Customer,
		Product:   input.Product,
		Quantity:  input.Quantity,
		UnitPrice: input.UnitPrice,
		Amount:    float64(input.Quantity) * input.UnitPrice,
		Address:   input.Address,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// MarkPaymentCompleted aplica el evento payment.completed.
func (o *Order) MarkPaymentCompleted() error {
	switch o.Status {
	case StatusPaymentCompleted:
		return nil
	case StatusPending:
		o.setStatus(StatusPaymentCompleted)
		return nil
	default:
		return o.transitionError(StatusPaymentCompleted)
	}
}

// MarkInventoryReserved aplica el evento inventory.reserved.
func (o *Order) MarkInventoryReserved() error {
	switch o.Status {
	case StatusInventoryReserved:
		return nil
	case StatusPaymentCompleted:
		o.setStatus(StatusInventoryReserved)
		return nil
	default:
		return o.transitionError(StatusInventoryReserved)
	}
}

// MarkCompleted aplica shipping.scheduled y cierra la saga como exitosa.
func (o *Order) MarkCompleted() error {
	switch o.Status {
	case StatusCompleted:
		return nil
	case StatusInventoryReserved:
		o.setStatus(StatusCompleted)
		return nil
	default:
		return o.transitionError(StatusCompleted)
	}
}

// MarkCompensating registra que la saga entró en compensación.
func (o *Order) MarkCompensating(reason string) error {
	switch o.Status {
	case StatusCompleted, StatusCancelled:
		return o.transitionError(StatusCompensating)
	default:
		o.Status = StatusCompensating
		if reason != "" {
			o.Reason = reason
		}
		o.UpdatedAt = time.Now().UTC()
		return nil
	}
}

// Cancel deja el pedido en estado final CANCELLED.
func (o *Order) Cancel(reason string) error {
	switch o.Status {
	case StatusCancelled:
		if reason != "" {
			o.Reason = reason
			o.UpdatedAt = time.Now().UTC()
		}
		return nil
	case StatusCompleted:
		return o.transitionError(StatusCancelled)
	default:
		o.Status = StatusCancelled
		o.Reason = reason
		o.UpdatedAt = time.Now().UTC()
		return nil
	}
}

// SetTracking guarda el número de seguimiento del envío.
func (o *Order) SetTracking(tracking string) {
	o.Tracking = tracking
	o.UpdatedAt = time.Now().UTC()
}

// IsFinal indica si el pedido ya no admite más transiciones de negocio.
func (o *Order) IsFinal() bool {
	return o.Status == StatusCompleted || o.Status == StatusCancelled
}

func (o *Order) setStatus(status Status) {
	o.Status = status
	o.Reason = ""
	o.UpdatedAt = time.Now().UTC()
}

func (o *Order) transitionError(target Status) error {
	return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, o.Status, target)
}
