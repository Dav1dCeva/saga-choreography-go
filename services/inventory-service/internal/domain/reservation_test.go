package domain

import (
	"errors"
	"testing"
)

func TestNewReservation(t *testing.T) {
	reservation, err := NewReservation("ORD-1", "ORD-1", "laptop", 2)
	if err != nil {
		t.Fatalf("NewReservation: %v", err)
	}
	if reservation.Status != StatusReserved || !reservation.IsReserved() {
		t.Fatalf("reserva inesperada: %+v", reservation)
	}
}

func TestNewReservationValidation(t *testing.T) {
	tests := []struct {
		name     string
		orderID  string
		sagaID   string
		sku      string
		quantity int
	}{
		{name: "sin order", orderID: "", sagaID: "s", sku: "x", quantity: 1},
		{name: "sin saga", orderID: "o", sagaID: "", sku: "x", quantity: 1},
		{name: "sin sku", orderID: "o", sagaID: "s", sku: " ", quantity: 1},
		{name: "cantidad cero", orderID: "o", sagaID: "s", sku: "x", quantity: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewReservation(test.orderID, test.sagaID, test.sku, test.quantity); !errors.Is(err, ErrInvalidReservation) {
				t.Fatalf("se esperaba ErrInvalidReservation, got %v", err)
			}
		})
	}
}

func TestRelease(t *testing.T) {
	reservation, _ := NewReservation("o", "s", "x", 1)
	if err := reservation.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if reservation.Status != StatusReleased || reservation.IsReserved() {
		t.Fatalf("reserva inesperada: %+v", reservation)
	}
	if err := reservation.Release(); err != nil {
		t.Fatalf("Release duplicado: %v", err)
	}

	reservation.Status = "DESCONOCIDO"
	if err := reservation.Release(); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("se esperaba ErrInvalidTransition, got %v", err)
	}
}
