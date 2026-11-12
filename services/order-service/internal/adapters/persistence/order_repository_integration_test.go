//go:build integration

package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"saga-choreography-go/pkg/postgres"
	"saga-choreography-go/services/order-service/internal/domain"
	"saga-choreography-go/services/order-service/internal/ports"
)

func TestOrderRepositoryIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_ORDER_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://saga:saga@localhost:5433/orders?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db, err := postgres.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("conectar postgres: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrar: %v", err)
	}

	orderID := "it-" + uuid.NewString()
	order, err := domain.NewOrder(domain.NewOrderInput{
		ID:        orderID,
		SagaID:    orderID,
		Customer:  "Cliente Integración",
		Product:   "phone",
		Quantity:  1,
		UnitPrice: 400,
		Address:   "Av. Siempre Viva 742",
	})
	if err != nil {
		t.Fatalf("crear dominio: %v", err)
	}
	defer func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM orders WHERE id = $1", orderID)
	}()

	repository := NewOrderRepository(db)
	if err := repository.Create(ctx, order); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repository.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != domain.StatusPending || got.Amount != 400 {
		t.Fatalf("pedido recuperado inesperado: %+v", got)
	}

	if err := got.MarkPaymentCompleted(); err != nil {
		t.Fatalf("MarkPaymentCompleted: %v", err)
	}
	if err := repository.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := repository.Get(ctx, orderID)
	if err != nil {
		t.Fatalf("Get actualizado: %v", err)
	}
	if updated.Status != domain.StatusPaymentCompleted {
		t.Fatalf("estado esperado %s, obtenido %s", domain.StatusPaymentCompleted, updated.Status)
	}

	orders, err := repository.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, item := range orders {
		if item.ID == orderID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("el pedido de integración no aparece en List")
	}

	if _, err := repository.Get(ctx, "no-existe-"+uuid.NewString()); err != ports.ErrNotFound {
		t.Fatalf("se esperaba ErrNotFound, se obtuvo %v", err)
	}
}
