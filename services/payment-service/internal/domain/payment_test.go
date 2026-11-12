package domain

import (
	"errors"
	"testing"
)

func TestNewPayment(t *testing.T) {
	payment, err := NewPayment("ORD-1", "ORD-1", 900)
	if err != nil {
		t.Fatalf("NewPayment: %v", err)
	}
	if payment.Status != StatusPending || payment.Amount != 900 {
		t.Fatalf("pago inesperado: %+v", payment)
	}
}

func TestNewPaymentValidation(t *testing.T) {
	tests := []struct {
		name    string
		orderID string
		sagaID  string
		amount  float64
	}{
		{name: "sin order", orderID: "", sagaID: "s", amount: 10},
		{name: "sin saga", orderID: "o", sagaID: "", amount: 10},
		{name: "monto negativo", orderID: "o", sagaID: "s", amount: -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewPayment(test.orderID, test.sagaID, test.amount); !errors.Is(err, ErrInvalidPayment) {
				t.Fatalf("se esperaba ErrInvalidPayment, got %v", err)
			}
		})
	}
}

func TestAuthorize(t *testing.T) {
	payment, _ := NewPayment("o", "s", 10)
	if err := payment.Authorize(); err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !payment.IsAuthorized() {
		t.Fatal("debió quedar autorizado")
	}
	if err := payment.Authorize(); err != nil {
		t.Fatalf("Authorize duplicado: %v", err)
	}

	payment2, _ := NewPayment("o2", "s2", 10)
	_ = payment2.Fail("x")
	if err := payment2.Authorize(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}
}

func TestFail(t *testing.T) {
	payment, _ := NewPayment("o", "s", 10)
	if err := payment.Fail("rechazado"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if payment.Status != StatusFailed || payment.Reason != "rechazado" {
		t.Fatalf("pago inesperado: %+v", payment)
	}
	if err := payment.Fail("otra vez"); err != nil {
		t.Fatalf("Fail duplicado: %v", err)
	}

	payment2, _ := NewPayment("o2", "s2", 10)
	_ = payment2.Authorize()
	if err := payment2.Fail("x"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}
}

func TestRefund(t *testing.T) {
	payment, _ := NewPayment("o", "s", 10)
	_ = payment.Authorize()
	if err := payment.Refund("compensación"); err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if payment.Status != StatusRefunded || payment.Reason != "compensación" {
		t.Fatalf("pago inesperado: %+v", payment)
	}
	if err := payment.Refund("otra"); err != nil {
		t.Fatalf("Refund duplicado: %v", err)
	}

	payment2, _ := NewPayment("o2", "s2", 10)
	if err := payment2.Refund("x"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}

	payment3, _ := NewPayment("o3", "s3", 10)
	_ = payment3.Fail("x")
	if err := payment3.Refund("x"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}
}
