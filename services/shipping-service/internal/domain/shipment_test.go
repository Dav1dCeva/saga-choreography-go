package domain

import (
	"errors"
	"testing"
)

func TestNewShipment(t *testing.T) {
	shipment, err := NewShipment("ORD-1", "ORD-1", "Av. Siempre Viva 742")
	if err != nil {
		t.Fatalf("NewShipment: %v", err)
	}
	if shipment.Status != StatusPending {
		t.Fatalf("estado = %s", shipment.Status)
	}
}

func TestNewShipmentValidation(t *testing.T) {
	if _, err := NewShipment("", "s", "x"); !errors.Is(err, ErrInvalidShipment) {
		t.Fatalf("se esperaba ErrInvalidShipment, got %v", err)
	}
	if _, err := NewShipment("o", "", "x"); !errors.Is(err, ErrInvalidShipment) {
		t.Fatalf("se esperaba ErrInvalidShipment, got %v", err)
	}
}

func TestScheduleAndFail(t *testing.T) {
	shipment, _ := NewShipment("o", "s", "dirección")
	if err := shipment.Schedule("TRK-1"); err != nil {
		t.Fatalf("Schedule: %v", err)
	}
	if shipment.Status != StatusScheduled || shipment.Tracking != "TRK-1" {
		t.Fatalf("envío inesperado: %+v", shipment)
	}
	if err := shipment.Schedule("TRK-2"); err != nil {
		t.Fatalf("Schedule duplicado: %v", err)
	}
	if err := shipment.Fail("x"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}

	shipment2, _ := NewShipment("o2", "s2", "dirección")
	if err := shipment2.Fail("inválida"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if shipment2.Status != StatusFailed || shipment2.Reason != "inválida" {
		t.Fatalf("envío inesperado: %+v", shipment2)
	}
	if err := shipment2.Fail("otra"); err != nil {
		t.Fatalf("Fail duplicado: %v", err)
	}
	if err := shipment2.Schedule("TRK"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}
}

func TestValidAddress(t *testing.T) {
	tests := []struct {
		address string
		valid   bool
	}{
		{address: "Av. Siempre Viva 742", valid: true},
		{address: "  calle 1  ", valid: true},
		{address: "", valid: false},
		{address: "   ", valid: false},
		{address: "direccion INVALID", valid: false},
	}
	for _, test := range tests {
		if got := ValidAddress(test.address); got != test.valid {
			t.Fatalf("ValidAddress(%q) = %v, se esperaba %v", test.address, got, test.valid)
		}
	}
}
