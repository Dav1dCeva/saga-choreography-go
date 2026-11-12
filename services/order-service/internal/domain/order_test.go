package domain

import (
	"errors"
	"testing"
)

func validInput() NewOrderInput {
	return NewOrderInput{
		ID:        "ORD-1",
		SagaID:    "ORD-1",
		Customer:  "Ana",
		Product:   "laptop",
		Quantity:  2,
		UnitPrice: 900,
		Address:   "Av. Siempre Viva 742",
	}
}

func TestNewOrder(t *testing.T) {
	order, err := NewOrder(validInput())
	if err != nil {
		t.Fatalf("NewOrder: %v", err)
	}
	if order.Amount != 1800 {
		t.Fatalf("amount = %v", order.Amount)
	}
	if order.Status != StatusPending {
		t.Fatalf("status = %s", order.Status)
	}
	if order.CreatedAt.IsZero() || order.UpdatedAt.IsZero() {
		t.Fatal("fechas sin inicializar")
	}
}

func TestNewOrderValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*NewOrderInput)
	}{
		{name: "sin id", mutate: func(i *NewOrderInput) { i.ID = "" }},
		{name: "sin saga", mutate: func(i *NewOrderInput) { i.SagaID = "" }},
		{name: "sin cliente", mutate: func(i *NewOrderInput) { i.Customer = "  " }},
		{name: "sin producto", mutate: func(i *NewOrderInput) { i.Product = "" }},
		{name: "cantidad cero", mutate: func(i *NewOrderInput) { i.Quantity = 0 }},
		{name: "precio cero", mutate: func(i *NewOrderInput) { i.UnitPrice = 0 }},
		{name: "sin dirección", mutate: func(i *NewOrderInput) { i.Address = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			test.mutate(&input)
			if _, err := NewOrder(input); !errors.Is(err, ErrInvalidOrder) {
				t.Fatalf("se esperaba ErrInvalidOrder, got %v", err)
			}
		})
	}
}

func TestOrderHappyTransitions(t *testing.T) {
	order, _ := NewOrder(validInput())

	if err := order.MarkPaymentCompleted(); err != nil {
		t.Fatalf("MarkPaymentCompleted: %v", err)
	}
	if order.Status != StatusPaymentCompleted {
		t.Fatalf("status = %s", order.Status)
	}
	if err := order.MarkPaymentCompleted(); err != nil {
		t.Fatalf("duplicado MarkPaymentCompleted: %v", err)
	}

	if err := order.MarkInventoryReserved(); err != nil {
		t.Fatalf("MarkInventoryReserved: %v", err)
	}
	if order.Status != StatusInventoryReserved {
		t.Fatalf("status = %s", order.Status)
	}
	if err := order.MarkInventoryReserved(); err != nil {
		t.Fatalf("duplicado MarkInventoryReserved: %v", err)
	}

	order.SetTracking("TRK-1")
	if err := order.MarkCompleted(); err != nil {
		t.Fatalf("MarkCompleted: %v", err)
	}
	if order.Status != StatusCompleted || !order.IsFinal() {
		t.Fatalf("estado final inesperado: %+v", order)
	}
	if err := order.MarkCompleted(); err != nil {
		t.Fatalf("duplicado MarkCompleted: %v", err)
	}
	if order.Tracking != "TRK-1" {
		t.Fatalf("tracking = %q", order.Tracking)
	}
}

func TestOrderInvalidTransitions(t *testing.T) {
	order, _ := NewOrder(validInput())
	if err := order.MarkInventoryReserved(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkInventoryReserved desde PENDING: %v", err)
	}
	if err := order.MarkCompleted(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkCompleted desde PENDING: %v", err)
	}

	_ = order.MarkPaymentCompleted()
	if err := order.MarkPaymentCompleted(); err != nil {
		t.Fatalf("duplicado: %v", err)
	}

	_ = order.MarkInventoryReserved()
	_ = order.MarkCompleted()
	if err := order.MarkCompensating("tarde"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("MarkCompensating desde COMPLETED: %v", err)
	}
	if err := order.Cancel("tarde"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Cancel desde COMPLETED: %v", err)
	}
}

func TestOrderCompensatingAndCancel(t *testing.T) {
	order, _ := NewOrder(validInput())
	if err := order.MarkCompensating("fallo de inventario"); err != nil {
		t.Fatalf("MarkCompensating: %v", err)
	}
	if order.Status != StatusCompensating || order.Reason != "fallo de inventario" {
		t.Fatalf("estado inesperado: %+v", order)
	}
	if err := order.Cancel("cancelado"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if order.Status != StatusCancelled || !order.IsFinal() {
		t.Fatalf("estado inesperado: %+v", order)
	}
	if err := order.Cancel("otra razón"); err != nil {
		t.Fatalf("Cancel duplicado: %v", err)
	}
	if order.Reason != "otra razón" {
		t.Fatalf("reason = %q", order.Reason)
	}
	if err := order.Cancel(""); err != nil {
		t.Fatalf("Cancel sin razón: %v", err)
	}
}

func TestMarkCompensatingFromCancelledFails(t *testing.T) {
	order, _ := NewOrder(validInput())
	_ = order.Cancel("x")
	if err := order.MarkCompensating("y"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}
}
