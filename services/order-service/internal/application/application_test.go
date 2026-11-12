package application

import (
	"context"
	"errors"
	"testing"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/order-service/internal/domain"
	"saga-choreography-go/services/order-service/internal/ports"
)

type fakeOrderRepository struct {
	orders    map[string]*domain.Order
	createErr error
	updateErr error
	getErr    error
	updates   int
}

func newFakeOrderRepository(orders ...*domain.Order) *fakeOrderRepository {
	repo := &fakeOrderRepository{orders: map[string]*domain.Order{}}
	for _, order := range orders {
		copy := *order
		repo.orders[order.ID] = &copy
	}
	return repo
}

func (r *fakeOrderRepository) Create(_ context.Context, order *domain.Order) error {
	if r.createErr != nil {
		return r.createErr
	}
	copy := *order
	r.orders[order.ID] = &copy
	return nil
}

func (r *fakeOrderRepository) Update(_ context.Context, order *domain.Order) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	copy := *order
	r.orders[order.ID] = &copy
	r.updates++
	return nil
}

func (r *fakeOrderRepository) Get(_ context.Context, id string) (*domain.Order, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	order, ok := r.orders[id]
	if !ok {
		return nil, ports.ErrNotFound
	}
	copy := *order
	return &copy, nil
}

func (r *fakeOrderRepository) List(_ context.Context) ([]domain.Order, error) {
	result := make([]domain.Order, 0, len(r.orders))
	for _, order := range r.orders {
		result = append(result, *order)
	}
	return result, nil
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

type fakeMetrics struct {
	started     int
	completed   int
	compensated int
}

func (m *fakeMetrics) SagaStarted()     { m.started++ }
func (m *fakeMetrics) SagaCompleted()   { m.completed++ }
func (m *fakeMetrics) SagaCompensated() { m.compensated++ }

func newOrder(t *testing.T) *domain.Order {
	t.Helper()
	order, err := domain.NewOrder(domain.NewOrderInput{
		ID:        "ORD-1",
		SagaID:    "ORD-1",
		Customer:  "Ana",
		Product:   "laptop",
		Quantity:  1,
		UnitPrice: 900,
		Address:   "Av. Siempre Viva 742",
	})
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	return order
}

func mustEvent(t *testing.T, eventType, orderID string, payload any) events.Event {
	t.Helper()
	event, err := events.New(eventType, orderID, orderID, payload)
	if err != nil {
		t.Fatalf("events.New: %v", err)
	}
	return event
}

func TestCreateOrderSuccess(t *testing.T) {
	repo := newFakeOrderRepository()
	publisher := &fakePublisher{}
	metrics := &fakeMetrics{}
	useCase := NewCreateOrder(repo, publisher, metrics, func() string { return "ORD-1" })

	order, err := useCase.Execute(context.Background(), CreateOrderCommand{
		Customer:  "Ana",
		Product:   "laptop",
		Quantity:  2,
		UnitPrice: 900,
		Address:   "Av. Siempre Viva 742",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if order.ID != "ORD-1" || order.Amount != 1800 {
		t.Fatalf("pedido inesperado: %+v", order)
	}
	if len(publisher.items) != 1 || publisher.items[0].routingKey != events.OrderCreated {
		t.Fatalf("publicación inesperada: %+v", publisher.items)
	}
	var payload OrderCreatedPayload
	if err := publisher.items[0].event.Decode(&payload); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if payload.Amount != 1800 || payload.Customer != "Ana" {
		t.Fatalf("payload inesperado: %+v", payload)
	}
	if metrics.started != 1 {
		t.Fatalf("SagaStarted = %d", metrics.started)
	}
}

func TestCreateOrderValidation(t *testing.T) {
	repo := newFakeOrderRepository()
	publisher := &fakePublisher{}
	useCase := NewCreateOrder(repo, publisher, &fakeMetrics{}, func() string { return "ORD-1" })

	_, err := useCase.Execute(context.Background(), CreateOrderCommand{Customer: "Ana"})
	if !errors.Is(err, domain.ErrInvalidOrder) {
		t.Fatalf("se esperaba ErrInvalidOrder, got %v", err)
	}
	if len(publisher.items) != 0 {
		t.Fatal("no debió publicar")
	}
}

func TestCreateOrderRepositoryError(t *testing.T) {
	repo := newFakeOrderRepository()
	repo.createErr = errors.New("db caída")
	useCase := NewCreateOrder(repo, &fakePublisher{}, &fakeMetrics{}, func() string { return "ORD-1" })

	if _, err := useCase.Execute(context.Background(), CreateOrderCommand{
		Customer: "Ana", Product: "laptop", Quantity: 1, UnitPrice: 10, Address: "x",
	}); err == nil {
		t.Fatal("se esperaba error")
	}
}

func TestCreateOrderPublisherError(t *testing.T) {
	publisher := &fakePublisher{err: errors.New("broker caído")}
	useCase := NewCreateOrder(newFakeOrderRepository(), publisher, &fakeMetrics{}, func() string { return "ORD-1" })

	if _, err := useCase.Execute(context.Background(), CreateOrderCommand{
		Customer: "Ana", Product: "laptop", Quantity: 1, UnitPrice: 10, Address: "x",
	}); err == nil {
		t.Fatal("se esperaba error")
	}
}

func TestEventHandlerHappyFlow(t *testing.T) {
	order := newOrder(t)
	repo := newFakeOrderRepository(order)
	metrics := &fakeMetrics{}
	handler := NewEventHandler(repo, metrics, nil)
	ctx := context.Background()

	if err := handler.OnPaymentCompleted(ctx, mustEvent(t, events.PaymentCompleted, order.ID, PaymentCompletedPayload{OrderID: order.ID})); err != nil {
		t.Fatalf("OnPaymentCompleted: %v", err)
	}
	if err := handler.OnInventoryReserved(ctx, mustEvent(t, events.InventoryReserved, order.ID, InventoryReservedPayload{OrderID: order.ID})); err != nil {
		t.Fatalf("OnInventoryReserved: %v", err)
	}
	if err := handler.OnShippingScheduled(ctx, mustEvent(t, events.ShippingScheduled, order.ID, ShippingScheduledPayload{OrderID: order.ID, Tracking: "TRK-1"})); err != nil {
		t.Fatalf("OnShippingScheduled: %v", err)
	}

	stored, _ := repo.Get(ctx, order.ID)
	if stored.Status != domain.StatusCompleted || stored.Tracking != "TRK-1" {
		t.Fatalf("estado inesperado: %+v", stored)
	}
	if metrics.completed != 1 {
		t.Fatalf("SagaCompleted = %d", metrics.completed)
	}
}

func TestEventHandlerPaymentFailed(t *testing.T) {
	order := newOrder(t)
	repo := newFakeOrderRepository(order)
	metrics := &fakeMetrics{}
	handler := NewEventHandler(repo, metrics, nil)
	event := mustEvent(t, events.PaymentFailed, order.ID, PaymentFailedPayload{OrderID: order.ID, Reason: "rechazado"})

	if err := handler.OnPaymentFailed(context.Background(), event); err != nil {
		t.Fatalf("OnPaymentFailed: %v", err)
	}
	stored, _ := repo.Get(context.Background(), order.ID)
	if stored.Status != domain.StatusCancelled || stored.Reason != "rechazado" {
		t.Fatalf("estado inesperado: %+v", stored)
	}
	if metrics.compensated != 1 {
		t.Fatalf("SagaCompensated = %d", metrics.compensated)
	}

	if err := handler.OnPaymentFailed(context.Background(), event); err != nil {
		t.Fatalf("duplicado: %v", err)
	}
	if metrics.compensated != 1 {
		t.Fatalf("no debió compensar dos veces: %d", metrics.compensated)
	}
}

func TestEventHandlerPaymentRefunded(t *testing.T) {
	order := newOrder(t)
	_ = order.MarkPaymentCompleted()
	repo := newFakeOrderRepository(order)
	metrics := &fakeMetrics{}
	handler := NewEventHandler(repo, metrics, nil)

	if err := handler.OnPaymentRefunded(context.Background(), mustEvent(t, events.PaymentRefunded, order.ID, PaymentRefundedPayload{OrderID: order.ID, Reason: "reembolsado"})); err != nil {
		t.Fatalf("OnPaymentRefunded: %v", err)
	}
	stored, _ := repo.Get(context.Background(), order.ID)
	if stored.Status != domain.StatusCancelled {
		t.Fatalf("estado = %s", stored.Status)
	}
	if metrics.compensated != 1 {
		t.Fatalf("SagaCompensated = %d", metrics.compensated)
	}
}

func TestEventHandlerCompensating(t *testing.T) {
	order := newOrder(t)
	repo := newFakeOrderRepository(order)
	handler := NewEventHandler(repo, &fakeMetrics{}, nil)

	if err := handler.OnInventoryFailed(context.Background(), mustEvent(t, events.InventoryFailed, order.ID, InventoryFailedPayload{OrderID: order.ID, Reason: "sin stock"})); err != nil {
		t.Fatalf("OnInventoryFailed: %v", err)
	}
	stored, _ := repo.Get(context.Background(), order.ID)
	if stored.Status != domain.StatusCompensating {
		t.Fatalf("estado = %s", stored.Status)
	}

	order2 := newOrder(t)
	order2.ID = "ORD-2"
	order2.SagaID = "ORD-2"
	_ = order2.MarkPaymentCompleted()
	_ = order2.MarkInventoryReserved()
	repo2 := newFakeOrderRepository(order2)
	handler2 := NewEventHandler(repo2, &fakeMetrics{}, nil)
	if err := handler2.OnShippingFailed(context.Background(), mustEvent(t, events.ShippingFailed, order2.ID, ShippingFailedPayload{OrderID: order2.ID, Reason: "envío"})); err != nil {
		t.Fatalf("OnShippingFailed: %v", err)
	}
	stored2, _ := repo2.Get(context.Background(), order2.ID)
	if stored2.Status != domain.StatusCompensating {
		t.Fatalf("estado = %s", stored2.Status)
	}
}

func TestEventHandlerIgnoresInvalidTransition(t *testing.T) {
	order := newOrder(t)
	repo := newFakeOrderRepository(order)
	metrics := &fakeMetrics{}
	handler := NewEventHandler(repo, metrics, nil)

	if err := handler.OnShippingScheduled(context.Background(), mustEvent(t, events.ShippingScheduled, order.ID, ShippingScheduledPayload{OrderID: order.ID})); err != nil {
		t.Fatalf("se esperaba nil, got %v", err)
	}
	if metrics.completed != 0 {
		t.Fatalf("SagaCompleted = %d", metrics.completed)
	}
}

func TestEventHandlerDecodeError(t *testing.T) {
	order := newOrder(t)
	handler := NewEventHandler(newFakeOrderRepository(order), &fakeMetrics{}, nil)
	bad := events.Event{EventID: "e", SagaID: order.ID, OrderID: order.ID, Type: events.PaymentFailed, Payload: []byte("{")}
	if err := handler.OnPaymentFailed(context.Background(), bad); err == nil {
		t.Fatal("se esperaba error de decode")
	}
}

func TestEventHandlerRepositoryErrors(t *testing.T) {
	order := newOrder(t)
	event := mustEvent(t, events.PaymentCompleted, order.ID, PaymentCompletedPayload{OrderID: order.ID})

	repo := newFakeOrderRepository(order)
	repo.getErr = errors.New("lectura")
	handler := NewEventHandler(repo, &fakeMetrics{}, nil)
	if err := handler.OnPaymentCompleted(context.Background(), event); err == nil {
		t.Fatal("se esperaba error de lectura")
	}

	repo = newFakeOrderRepository(order)
	repo.updateErr = errors.New("escritura")
	handler = NewEventHandler(repo, &fakeMetrics{}, nil)
	if err := handler.OnPaymentCompleted(context.Background(), event); err == nil {
		t.Fatal("se esperaba error de escritura")
	}
}
