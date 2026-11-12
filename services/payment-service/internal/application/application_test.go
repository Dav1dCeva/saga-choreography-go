package application

import (
	"context"
	"errors"
	"testing"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/payment-service/internal/domain"
	"saga-choreography-go/services/payment-service/internal/ports"
)

type fakePaymentRepository struct {
	payments map[string]*domain.Payment
	saveErr  error
	getErr   error
}

func newFakePaymentRepository(payments ...*domain.Payment) *fakePaymentRepository {
	repo := &fakePaymentRepository{payments: map[string]*domain.Payment{}}
	for _, payment := range payments {
		copy := *payment
		repo.payments[payment.OrderID] = &copy
	}
	return repo
}

func (r *fakePaymentRepository) Save(_ context.Context, payment *domain.Payment) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	copy := *payment
	r.payments[payment.OrderID] = &copy
	return nil
}

func (r *fakePaymentRepository) Get(_ context.Context, orderID string) (*domain.Payment, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	payment, ok := r.payments[orderID]
	if !ok {
		return nil, ports.ErrNotFound
	}
	copy := *payment
	return &copy, nil
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

func orderCreatedEvent(t *testing.T, payload OrderCreatedPayload) events.Event {
	t.Helper()
	event, err := events.New(events.OrderCreated, payload.OrderID, payload.OrderID, payload)
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	return event
}

func defaultPayload() OrderCreatedPayload {
	return OrderCreatedPayload{
		OrderID:   "ORD-1",
		Customer:  "Ana",
		Product:   "laptop",
		Quantity:  1,
		UnitPrice: 900,
		Amount:    900,
		Address:   "Av. Siempre Viva 742",
	}
}

func TestChargeOrderSuccess(t *testing.T) {
	repo := newFakePaymentRepository()
	publisher := &fakePublisher{}
	useCase := NewChargeOrder(repo, publisher, ChargeConfig{MaxAmount: 1000, BlockedCustomer: "blocked"}, nil)

	if err := useCase.Handle(context.Background(), orderCreatedEvent(t, defaultPayload())); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	payment, err := repo.Get(context.Background(), "ORD-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if payment.Status != domain.StatusAuthorized {
		t.Fatalf("estado = %s", payment.Status)
	}
	if len(publisher.items) != 1 || publisher.items[0].routingKey != events.PaymentCompleted {
		t.Fatalf("publicación inesperada: %+v", publisher.items)
	}
	var payload PaymentCompletedPayload
	if err := publisher.items[0].event.Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if payload.Product != "laptop" || payload.Address == "" {
		t.Fatalf("payload inesperado: %+v", payload)
	}
}

func TestChargeOrderFailures(t *testing.T) {
	tests := []struct {
		name   string
		config ChargeConfig
		change func(*OrderCreatedPayload)
	}{
		{
			name:   "fallo inyectado",
			config: ChargeConfig{FailPayment: true, MaxAmount: 1000},
			change: func(_ *OrderCreatedPayload) {},
		},
		{
			name:   "monto superior",
			config: ChargeConfig{MaxAmount: 1000},
			change: func(p *OrderCreatedPayload) { p.Amount = 1500 },
		},
		{
			name:   "cliente bloqueado",
			config: ChargeConfig{MaxAmount: 1000, BlockedCustomer: "blocked"},
			change: func(p *OrderCreatedPayload) { p.Customer = "blocked" },
		},
		{
			name:   "monto inválido",
			config: ChargeConfig{MaxAmount: 1000},
			change: func(p *OrderCreatedPayload) { p.Amount = 0 },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := defaultPayload()
			test.change(&payload)
			repo := newFakePaymentRepository()
			publisher := &fakePublisher{}
			useCase := NewChargeOrder(repo, publisher, test.config, nil)
			if err := useCase.Handle(context.Background(), orderCreatedEvent(t, payload)); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			payment, _ := repo.Get(context.Background(), payload.OrderID)
			if payment.Status != domain.StatusFailed {
				t.Fatalf("estado = %s", payment.Status)
			}
			if len(publisher.items) != 1 || publisher.items[0].routingKey != events.PaymentFailed {
				t.Fatalf("publicación inesperada: %+v", publisher.items)
			}
		})
	}
}

func TestChargeOrderDecodeError(t *testing.T) {
	useCase := NewChargeOrder(newFakePaymentRepository(), &fakePublisher{}, ChargeConfig{}, nil)
	bad := events.Event{EventID: "e", SagaID: "s", OrderID: "o", Type: events.OrderCreated, Payload: []byte("{")}
	if err := useCase.Handle(context.Background(), bad); err == nil {
		t.Fatal("se esperaba error de decode")
	}
}

func TestChargeOrderInvalidPayment(t *testing.T) {
	useCase := NewChargeOrder(newFakePaymentRepository(), &fakePublisher{}, ChargeConfig{}, nil)
	event := events.Event{
		EventID: "e", SagaID: "", OrderID: "", Type: events.OrderCreated,
		Payload: []byte(`{"order_id":"","amount":10}`),
	}
	if err := useCase.Handle(context.Background(), event); err == nil {
		t.Fatal("se esperaba error de pago inválido")
	}
}

func TestChargeOrderSaveAndPublishErrors(t *testing.T) {
	repo := newFakePaymentRepository()
	repo.saveErr = errors.New("db")
	useCase := NewChargeOrder(repo, &fakePublisher{}, ChargeConfig{}, nil)
	if err := useCase.Handle(context.Background(), orderCreatedEvent(t, defaultPayload())); err == nil {
		t.Fatal("se esperaba error al guardar")
	}

	publisher := &fakePublisher{err: errors.New("broker")}
	useCase = NewChargeOrder(newFakePaymentRepository(), publisher, ChargeConfig{}, nil)
	if err := useCase.Handle(context.Background(), orderCreatedEvent(t, defaultPayload())); err == nil {
		t.Fatal("se esperaba error al publicar")
	}
}

func authorizedPayment(t *testing.T) *domain.Payment {
	t.Helper()
	payment, err := domain.NewPayment("ORD-1", "ORD-1", 900)
	if err != nil {
		t.Fatalf("NewPayment: %v", err)
	}
	_ = payment.Authorize()
	return payment
}

func compensationEvent(t *testing.T, eventType string, reason string) events.Event {
	t.Helper()
	event, err := events.New(eventType, "ORD-1", "ORD-1", map[string]string{"order_id": "ORD-1", "reason": reason})
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	return event
}

func TestRefundPayment(t *testing.T) {
	repo := newFakePaymentRepository(authorizedPayment(t))
	publisher := &fakePublisher{}
	useCase := NewRefundPayment(repo, publisher, nil)

	if err := useCase.Handle(context.Background(), compensationEvent(t, events.InventoryFailed, "sin stock"), "default"); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	payment, _ := repo.Get(context.Background(), "ORD-1")
	if payment.Status != domain.StatusRefunded || payment.Reason != "sin stock" {
		t.Fatalf("pago inesperado: %+v", payment)
	}
	if len(publisher.items) != 1 || publisher.items[0].routingKey != events.PaymentRefunded {
		t.Fatalf("publicación inesperada: %+v", publisher.items)
	}
}

func TestRefundPaymentSkips(t *testing.T) {
	tests := []struct {
		name string
		repo *fakePaymentRepository
	}{
		{name: "no existe", repo: newFakePaymentRepository()},
		{name: "no autorizado", repo: func() *fakePaymentRepository {
			payment, _ := domain.NewPayment("ORD-1", "ORD-1", 10)
			_ = payment.Fail("x")
			return newFakePaymentRepository(payment)
		}()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			publisher := &fakePublisher{}
			useCase := NewRefundPayment(test.repo, publisher, nil)
			if err := useCase.Handle(context.Background(), compensationEvent(t, events.InventoryFailed, ""), "default"); err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if len(publisher.items) != 0 {
				t.Fatalf("no debió publicar: %+v", publisher.items)
			}
		})
	}
}

func TestRefundPaymentErrors(t *testing.T) {
	repo := newFakePaymentRepository(authorizedPayment(t))
	repo.getErr = errors.New("db")
	useCase := NewRefundPayment(repo, &fakePublisher{}, nil)
	if err := useCase.Handle(context.Background(), compensationEvent(t, events.InventoryFailed, ""), "default"); err == nil {
		t.Fatal("se esperaba error de lectura")
	}

	repo = newFakePaymentRepository(authorizedPayment(t))
	repo.saveErr = errors.New("db")
	useCase = NewRefundPayment(repo, &fakePublisher{}, nil)
	if err := useCase.Handle(context.Background(), compensationEvent(t, events.InventoryFailed, ""), "default"); err == nil {
		t.Fatal("se esperaba error de escritura")
	}

	repo = newFakePaymentRepository(authorizedPayment(t))
	useCase = NewRefundPayment(repo, &fakePublisher{err: errors.New("broker")}, nil)
	if err := useCase.Handle(context.Background(), compensationEvent(t, events.InventoryFailed, ""), "default"); err == nil {
		t.Fatal("se esperaba error de publicación")
	}
}
