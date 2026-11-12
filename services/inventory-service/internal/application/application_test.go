package application

import (
	"context"
	"errors"
	"testing"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/inventory-service/internal/domain"
	"saga-choreography-go/services/inventory-service/internal/ports"
)

type fakeInventoryRepository struct {
	available     int
	reserveCreate bool
	reserveErr    error
	releaseResult bool
	releaseErr    error
	availableErr  error
	reservations  []*domain.Reservation
}

func (r *fakeInventoryRepository) Reserve(_ context.Context, reservation *domain.Reservation) (bool, error) {
	if r.reserveErr != nil {
		return false, r.reserveErr
	}
	if !r.reserveCreate {
		return false, nil
	}
	copy := *reservation
	r.reservations = append(r.reservations, &copy)
	return true, nil
}

func (r *fakeInventoryRepository) Release(_ context.Context, _ string) (bool, error) {
	if r.releaseErr != nil {
		return false, r.releaseErr
	}
	return r.releaseResult, nil
}

func (r *fakeInventoryRepository) Available(_ context.Context, _ string) (int, error) {
	if r.availableErr != nil {
		return 0, r.availableErr
	}
	return r.available, nil
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

func paymentCompletedEvent(t *testing.T, payload PaymentCompletedPayload) events.Event {
	t.Helper()
	event, err := events.New(events.PaymentCompleted, payload.OrderID, payload.OrderID, payload)
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	return event
}

func defaultPaymentPayload() PaymentCompletedPayload {
	return PaymentCompletedPayload{
		OrderID:  "ORD-1",
		Product:  "laptop",
		Quantity: 1,
		Amount:   900,
		Address:  "Av. Siempre Viva 742",
	}
}

func TestReserveStockSuccess(t *testing.T) {
	repo := &fakeInventoryRepository{reserveCreate: true}
	publisher := &fakePublisher{}
	useCase := NewReserveStock(repo, publisher, false, nil)

	if err := useCase.Handle(context.Background(), paymentCompletedEvent(t, defaultPaymentPayload())); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(repo.reservations) != 1 || repo.reservations[0].SKU != "laptop" {
		t.Fatalf("reservas inesperadas: %+v", repo.reservations)
	}
	if len(publisher.items) != 1 || publisher.items[0].routingKey != events.InventoryReserved {
		t.Fatalf("publicación inesperada: %+v", publisher.items)
	}
}

func TestReserveStockFailures(t *testing.T) {
	tests := []struct {
		name   string
		repo   *fakeInventoryRepository
		fail   bool
		mutate func(*PaymentCompletedPayload)
	}{
		{name: "fallo inyectado", repo: &fakeInventoryRepository{}, fail: true, mutate: func(_ *PaymentCompletedPayload) {}},
		{name: "sku inválido", repo: &fakeInventoryRepository{}, mutate: func(p *PaymentCompletedPayload) { p.Product = "" }},
		{name: "sin stock", repo: &fakeInventoryRepository{reserveErr: domain.ErrInsufficientStock, available: 2}, mutate: func(_ *PaymentCompletedPayload) {}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := defaultPaymentPayload()
			test.mutate(&payload)
			publisher := &fakePublisher{}
			useCase := NewReserveStock(test.repo, publisher, test.fail, nil)
			if err := useCase.Handle(context.Background(), paymentCompletedEvent(t, payload)); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(publisher.items) != 1 || publisher.items[0].routingKey != events.InventoryFailed {
				t.Fatalf("publicación inesperada: %+v", publisher.items)
			}
		})
	}
}

func TestReserveStockDuplicate(t *testing.T) {
	repo := &fakeInventoryRepository{reserveCreate: false}
	publisher := &fakePublisher{}
	useCase := NewReserveStock(repo, publisher, false, nil)
	if err := useCase.Handle(context.Background(), paymentCompletedEvent(t, defaultPaymentPayload())); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(publisher.items) != 0 {
		t.Fatalf("no debió publicar: %+v", publisher.items)
	}
}

func TestReserveStockErrors(t *testing.T) {
	repo := &fakeInventoryRepository{reserveErr: errors.New("db")}
	useCase := NewReserveStock(repo, &fakePublisher{}, false, nil)
	if err := useCase.Handle(context.Background(), paymentCompletedEvent(t, defaultPaymentPayload())); err == nil {
		t.Fatal("se esperaba error de repositorio")
	}

	repo = &fakeInventoryRepository{reserveCreate: true}
	useCase = NewReserveStock(repo, &fakePublisher{err: errors.New("broker")}, false, nil)
	if err := useCase.Handle(context.Background(), paymentCompletedEvent(t, defaultPaymentPayload())); err == nil {
		t.Fatal("se esperaba error de publicación")
	}

	bad := events.Event{EventID: "e", SagaID: "s", OrderID: "o", Type: events.PaymentCompleted, Payload: []byte("{")}
	useCase = NewReserveStock(&fakeInventoryRepository{}, &fakePublisher{}, false, nil)
	if err := useCase.Handle(context.Background(), bad); err == nil {
		t.Fatal("se esperaba error de decode")
	}
}

func shippingFailedEvent(t *testing.T) events.Event {
	t.Helper()
	event, err := events.New(events.ShippingFailed, "ORD-1", "ORD-1", ShippingFailedPayload{OrderID: "ORD-1", Reason: "envío fallido"})
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	return event
}

func TestReleaseStockSuccess(t *testing.T) {
	repo := &fakeInventoryRepository{releaseResult: true}
	publisher := &fakePublisher{}
	useCase := NewReleaseStock(repo, publisher, nil)

	if err := useCase.Handle(context.Background(), shippingFailedEvent(t)); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(publisher.items) != 1 || publisher.items[0].routingKey != events.InventoryReleased {
		t.Fatalf("publicación inesperada: %+v", publisher.items)
	}
	var payload InventoryReleasedPayload
	if err := publisher.items[0].event.Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if payload.Reason != "envío fallido" {
		t.Fatalf("reason = %q", payload.Reason)
	}
}

func TestReleaseStockSkips(t *testing.T) {
	tests := []struct {
		name string
		repo *fakeInventoryRepository
	}{
		{name: "sin reserva", repo: &fakeInventoryRepository{releaseErr: ports.ErrNotFound}},
		{name: "ya liberada", repo: &fakeInventoryRepository{releaseResult: false}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			publisher := &fakePublisher{}
			useCase := NewReleaseStock(test.repo, publisher, nil)
			if err := useCase.Handle(context.Background(), shippingFailedEvent(t)); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(publisher.items) != 0 {
				t.Fatalf("no debió publicar: %+v", publisher.items)
			}
		})
	}
}

func TestReleaseStockErrors(t *testing.T) {
	repo := &fakeInventoryRepository{releaseErr: errors.New("db")}
	useCase := NewReleaseStock(repo, &fakePublisher{}, nil)
	if err := useCase.Handle(context.Background(), shippingFailedEvent(t)); err == nil {
		t.Fatal("se esperaba error de repositorio")
	}

	repo = &fakeInventoryRepository{releaseResult: true}
	useCase = NewReleaseStock(repo, &fakePublisher{err: errors.New("broker")}, nil)
	if err := useCase.Handle(context.Background(), shippingFailedEvent(t)); err == nil {
		t.Fatal("se esperaba error de publicación")
	}
}

func TestReleaseStockDefaultReason(t *testing.T) {
	repo := &fakeInventoryRepository{releaseResult: true}
	publisher := &fakePublisher{}
	useCase := NewReleaseStock(repo, publisher, nil)
	event := events.Event{EventID: "e", SagaID: "s", OrderID: "o", Type: events.ShippingFailed, Payload: []byte(`{"reason":""}`)}
	if err := useCase.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var payload InventoryReleasedPayload
	if err := publisher.items[0].event.Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if payload.Reason == "" {
		t.Fatal("debió usar razón por defecto")
	}
}
