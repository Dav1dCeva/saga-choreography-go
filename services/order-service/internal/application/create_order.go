// Package application implementa los casos de uso del servicio de pedidos.
package application

import (
	"context"
	"fmt"

	"saga-choreography-go/pkg/events"
	"saga-choreography-go/services/order-service/internal/domain"
	"saga-choreography-go/services/order-service/internal/ports"
)

// CreateOrderCommand contiene los datos que llegan por REST.
type CreateOrderCommand struct {
	Customer  string
	Product   string
	Quantity  int
	UnitPrice float64
	Address   string
}

// CreateOrder crea el pedido y publica order.created.
type CreateOrder struct {
	repository ports.OrderRepository
	publisher  ports.EventPublisher
	metrics    ports.Metrics
	newID      func() string
}

// NewCreateOrder construye el caso de uso.
func NewCreateOrder(repository ports.OrderRepository, publisher ports.EventPublisher, metrics ports.Metrics, newID func() string) *CreateOrder {
	if newID == nil {
		newID = func() string { return "" }
	}
	return &CreateOrder{repository: repository, publisher: publisher, metrics: metrics, newID: newID}
}

// Execute valida, persiste y publica el primer evento de la coreografía.
func (uc *CreateOrder) Execute(ctx context.Context, command CreateOrderCommand) (*domain.Order, error) {
	orderID := uc.newID()
	order, err := domain.NewOrder(domain.NewOrderInput{
		ID:        orderID,
		SagaID:    orderID,
		Customer:  command.Customer,
		Product:   command.Product,
		Quantity:  command.Quantity,
		UnitPrice: command.UnitPrice,
		Address:   command.Address,
	})
	if err != nil {
		return nil, err
	}

	if err := uc.repository.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("guardar pedido: %w", err)
	}

	payload := OrderCreatedPayload{
		OrderID:   order.ID,
		Customer:  order.Customer,
		Product:   order.Product,
		Quantity:  order.Quantity,
		UnitPrice: order.UnitPrice,
		Amount:    order.Amount,
		Address:   order.Address,
	}
	event, err := events.New(events.OrderCreated, order.SagaID, order.ID, payload)
	if err != nil {
		return nil, err
	}
	if err := uc.publisher.Publish(ctx, events.OrderCreated, event); err != nil {
		return nil, fmt.Errorf("publicar order.created: %w", err)
	}

	uc.metrics.SagaStarted()
	return order, nil
}
