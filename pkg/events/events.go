// Package events define el sobre común que viaja por RabbitMQ y las routing
// keys de la coreografía. Es un paquete compartido por todos los servicios,
// pero no depende de RabbitMQ ni de la base de datos.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Routing keys utilizadas en el exchange topic saga.events.
const (
	OrderCreated      = "order.created"
	PaymentCompleted  = "payment.completed"
	PaymentFailed     = "payment.failed"
	PaymentRefunded   = "payment.refunded"
	InventoryReserved = "inventory.reserved"
	InventoryFailed   = "inventory.failed"
	InventoryReleased = "inventory.released"
	ShippingScheduled = "shipping.scheduled"
	ShippingFailed    = "shipping.failed"
)

// AllRoutingKeys agrupa todas las routing keys para documentación y pruebas.
var AllRoutingKeys = []string{
	OrderCreated,
	PaymentCompleted,
	PaymentFailed,
	PaymentRefunded,
	InventoryReserved,
	InventoryFailed,
	InventoryReleased,
	ShippingScheduled,
	ShippingFailed,
}

// ErrInvalidEvent indica que el sobre del evento está incompleto.
var ErrInvalidEvent = errors.New("evento inválido")

// Event es el sobre que comparten todos los mensajes de la saga.
//
// EventID permite la idempotencia, SagaID correlaciona toda la traza y
// OrderID actúa como identificador de negocio.
type Event struct {
	EventID   string          `json:"event_id"`
	SagaID    string          `json:"saga_id"`
	OrderID   string          `json:"order_id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// New construye un evento con UUID y timestamp UTC.
func New(eventType, sagaID, orderID string, payload any) (Event, error) {
	if eventType == "" {
		return Event{}, fmt.Errorf("%w: type vacío", ErrInvalidEvent)
	}
	if orderID == "" {
		return Event{}, fmt.Errorf("%w: order_id vacío", ErrInvalidEvent)
	}
	if sagaID == "" {
		sagaID = orderID
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("serializar payload: %w", err)
	}

	return Event{
		EventID:   uuid.NewString(),
		SagaID:    sagaID,
		OrderID:   orderID,
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   raw,
	}, nil
}

// Decode deserializa el payload en la estructura indicada.
func (e Event) Decode(target any) error {
	if len(e.Payload) == 0 {
		return fmt.Errorf("%w: payload vacío", ErrInvalidEvent)
	}
	return json.Unmarshal(e.Payload, target)
}

// Validate comprueba los campos mínimos del sobre.
func (e Event) Validate() error {
	switch {
	case e.EventID == "":
		return fmt.Errorf("%w: event_id vacío", ErrInvalidEvent)
	case e.Type == "":
		return fmt.Errorf("%w: type vacío", ErrInvalidEvent)
	case e.OrderID == "":
		return fmt.Errorf("%w: order_id vacío", ErrInvalidEvent)
	case e.Timestamp.IsZero():
		return fmt.Errorf("%w: timestamp vacío", ErrInvalidEvent)
	default:
		return nil
	}
}

// Saga devuelve el identificador de saga, usando el pedido como respaldo.
func (e Event) Saga() string {
	if e.SagaID != "" {
		return e.SagaID
	}
	return e.OrderID
}
