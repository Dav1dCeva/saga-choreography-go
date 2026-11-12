package events

import (
	"errors"
	"testing"
	"time"
)

type samplePayload struct {
	OrderID string `json:"order_id"`
	Amount  int    `json:"amount"`
}

func TestNewBuildsEnvelope(t *testing.T) {
	event, err := New(OrderCreated, "saga-1", "order-1", samplePayload{OrderID: "order-1", Amount: 10})
	if err != nil {
		t.Fatalf("New devolvió error: %v", err)
	}
	if event.EventID == "" || event.SagaID != "saga-1" || event.OrderID != "order-1" {
		t.Fatalf("sobre inesperado: %+v", event)
	}
	if event.Type != OrderCreated {
		t.Fatalf("type = %q", event.Type)
	}
	if event.Timestamp.IsZero() {
		t.Fatal("timestamp vacío")
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	var payload samplePayload
	if err := event.Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if payload.OrderID != "order-1" || payload.Amount != 10 {
		t.Fatalf("payload inesperado: %+v", payload)
	}
}

func TestNewUsesOrderIDAsSagaFallback(t *testing.T) {
	event, err := New(PaymentCompleted, "", "order-9", samplePayload{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if event.SagaID != "order-9" || event.Saga() != "order-9" {
		t.Fatalf("saga de respaldo incorrecta: %+v", event)
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		orderID   string
	}{
		{name: "sin type", eventType: "", orderID: "order-1"},
		{name: "sin order", eventType: OrderCreated, orderID: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.eventType, "saga", test.orderID, samplePayload{})
			if !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("se esperaba ErrInvalidEvent, got %v", err)
			}
		})
	}
}

func TestDecodeRejectsEmptyPayload(t *testing.T) {
	event := Event{EventID: "1", Type: OrderCreated, OrderID: "o", Timestamp: time.Now()}
	if err := event.Decode(&samplePayload{}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("se esperaba ErrInvalidEvent, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	valid := Event{EventID: "e", Type: OrderCreated, OrderID: "o", Timestamp: time.Now()}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate válido: %v", err)
	}

	tests := []struct {
		name  string
		event Event
	}{
		{name: "sin event id", event: Event{Type: OrderCreated, OrderID: "o", Timestamp: time.Now()}},
		{name: "sin type", event: Event{EventID: "e", OrderID: "o", Timestamp: time.Now()}},
		{name: "sin order", event: Event{EventID: "e", Type: OrderCreated, Timestamp: time.Now()}},
		{name: "sin timestamp", event: Event{EventID: "e", Type: OrderCreated, OrderID: "o"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.event.Validate(); !errors.Is(err, ErrInvalidEvent) {
				t.Fatalf("se esperaba ErrInvalidEvent, got %v", err)
			}
		})
	}
}

func TestSagaFallback(t *testing.T) {
	event := Event{OrderID: "order-x"}
	if event.Saga() != "order-x" {
		t.Fatalf("Saga() = %q", event.Saga())
	}
}
