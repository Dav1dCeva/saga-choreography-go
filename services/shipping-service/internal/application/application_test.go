package application

import (
	"context"
	"errors"
	"testing"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/shipping-service/internal/domain"
)

type fakeShipmentRepository struct {
	shipments []*domain.Shipment
	err       error
}

func (r *fakeShipmentRepository) Save(_ context.Context, shipment *domain.Shipment) error {
	if r.err != nil {
		return r.err
	}
	copy := *shipment
	r.shipments = append(r.shipments, &copy)
	return nil
}

type publishedEvent struct {
	routingKey string
	event      events.Event
}

type fakePublisher struct {
	items []publishedEvent
	err   error
}

func (p *fakePublisher) Publish(_ context.Context, routingKey string, event events.Event) error {
	if p.err != nil {
		return p.err
	}
	p.items = append(p.items, publishedEvent{routingKey: routingKey, event: event})
	return nil
}

func inventoryReservedEvent(t *testing.T, address string) events.Event {
	t.Helper()
	event, err := events.New(events.InventoryReserved, "ORD-1", "ORD-1", InventoryReservedPayload{
		OrderID:  "ORD-1",
		Product:  "laptop",
		Quantity: 1,
		Address:  address,
	})
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	return event
}

func TestScheduleShipmentSuccess(t *testing.T) {
	repo := &fakeShipmentRepository{}
	publisher := &fakePublisher{}
	useCase := NewScheduleShipment(repo, publisher, false, nil)

	if err := useCase.Handle(context.Background(), inventoryReservedEvent(t, "Av. Siempre Viva 742")); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(repo.shipments) != 1 || repo.shipments[0].Status != domain.StatusScheduled {
		t.Fatalf("envíos inesperados: %+v", repo.shipments)
	}
	if len(publisher.items) != 1 || publisher.items[0].routingKey != events.ShippingScheduled {
		t.Fatalf("publicación inesperada: %+v", publisher.items)
	}
	var payload ShippingScheduledPayload
	if err := publisher.items[0].event.Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if payload.Tracking != "TRK-ORD-1" {
		t.Fatalf("tracking = %q", payload.Tracking)
	}
}

func TestScheduleShipmentFailures(t *testing.T) {
	tests := []struct {
		name    string
		fail    bool
		address string
	}{
		{name: "fallo inyectado", fail: true, address: "Av. Siempre Viva 742"},
		{name: "dirección inválida", address: "direccion invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeShipmentRepository{}
			publisher := &fakePublisher{}
			useCase := NewScheduleShipment(repo, publisher, test.fail, nil)
			if err := useCase.Handle(context.Background(), inventoryReservedEvent(t, test.address)); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(repo.shipments) != 1 || repo.shipments[0].Status != domain.StatusFailed {
				t.Fatalf("envíos inesperados: %+v", repo.shipments)
			}
			if len(publisher.items) != 1 || publisher.items[0].routingKey != events.ShippingFailed {
				t.Fatalf("publicación inesperada: %+v", publisher.items)
			}
		})
	}
}

func TestScheduleShipmentErrors(t *testing.T) {
	bad := events.Event{EventID: "e", SagaID: "s", OrderID: "o", Type: events.InventoryReserved, Payload: []byte("{")}
	useCase := NewScheduleShipment(&fakeShipmentRepository{}, &fakePublisher{}, false, nil)
	if err := useCase.Handle(context.Background(), bad); err == nil {
		t.Fatal("se esperaba error de decode")
	}

	invalid := events.Event{EventID: "e", SagaID: "", OrderID: "", Type: events.InventoryReserved, Payload: []byte(`{"address":"x"}`)}
	if err := useCase.Handle(context.Background(), invalid); err == nil {
		t.Fatal("se esperaba error de envío inválido")
	}

	repo := &fakeShipmentRepository{err: errors.New("db")}
	useCase = NewScheduleShipment(repo, &fakePublisher{}, false, nil)
	if err := useCase.Handle(context.Background(), inventoryReservedEvent(t, "Av. Siempre Viva 742")); err == nil {
		t.Fatal("se esperaba error de repositorio")
	}

	useCase = NewScheduleShipment(&fakeShipmentRepository{}, &fakePublisher{err: errors.New("broker")}, false, nil)
	if err := useCase.Handle(context.Background(), inventoryReservedEvent(t, "Av. Siempre Viva 742")); err == nil {
		t.Fatal("se esperaba error de publicación")
	}
}
