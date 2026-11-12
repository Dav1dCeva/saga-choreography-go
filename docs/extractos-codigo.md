# Extractos de código representativos

Los fragmentos siguientes explican las decisiones centrales de la solución.
Los archivos completos están enlazados con su ruta dentro del repositorio.

## 1. Sobre común del evento y correlación

`pkg/events/events.go`

```go
const (
	OrderCreated      = "order.created"
	PaymentCompleted  = "payment.completed"
	PaymentFailed     = "payment.failed"
	PaymentRefunded   = "payment.refunded"
	InventoryReserved = "inventory.reserved"
	InventoryFailed   = "inventory.failed"
	InventoryReleased = "inventory.released"
	ShippingScheduled = "shipping.scheduled"
	ShippingFailed    = "shipping.failed"
)

type Event struct {
	EventID   string          `json:"event_id"`
	SagaID    string          `json:"saga_id"`
	OrderID   string          `json:"order_id"`
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}
```

Todos los mensajes comparten el mismo sobre. `event_id` permite idempotencia,
`saga_id` correlaciona la traza completa y `order_id` identifica el agregado de
negocio. `New` genera el UUID y el timestamp UTC, y usa `order_id` como
`saga_id` de respaldo si no se indica otro.

## 2. Consumidor con idempotencia, reintentos y DLQ

`pkg/broker/consumer.go`

```go
if c.cfg.Idempotency != nil {
	isNew, err := c.cfg.Idempotency.MarkProcessed(ctx, event.EventID, event.Type)
	if err != nil {
		c.fail()
		_ = delivery.Nack(false, true)
		return
	}
	if !isNew {
		c.logf("info", "evento duplicado ignorado", "event_id", event.EventID)
		_ = delivery.Ack(false)
		return
	}
}

handlerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
err := c.retry(handlerCtx, event)
cancel()
if err != nil {
	c.fail()
	if c.cfg.Idempotency != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = c.cfg.Idempotency.UnmarkProcessed(cleanupCtx, event.EventID)
		cleanupCancel()
	}
	_ = delivery.Nack(false, false) // va a la DLQ
	return
}
_ = delivery.Ack(false)
```

El consumidor usa **ack manual** y `prefetch=1`. Antes de ejecutar el handler
marca el `event_id`; si ya existía, hace `ack` y lo ignora. Si el handler falla
tras los reintentos, desmarca el evento y hace `nack` sin requeue para que
RabbitMQ lo enrute a `saga.dlx` y a `<cola>.dlq`.

## 3. Coreografía: inventario reacciona y publica el siguiente evento

`services/inventory-service/internal/application/reserve_stock.go`

```go
func (uc *ReserveStock) Handle(ctx context.Context, event events.Event) error {
	var payload PaymentCompletedPayload
	if err := event.Decode(&payload); err != nil {
		return err
	}

	if uc.fail {
		return uc.failEvent(ctx, event, payload.Product,
			"fallo inyectado por FAIL_INVENTORY=true")
	}

	reservation, err := domain.NewReservation(
		event.OrderID, event.Saga(), payload.Product, payload.Quantity,
	)
	if err != nil {
		return uc.failEvent(ctx, event, payload.Product, err.Error())
	}

	created, err := uc.repository.Reserve(ctx, reservation)
	if errors.Is(err, domain.ErrInsufficientStock) {
		available, availableErr := uc.repository.Available(ctx, payload.Product)
		if availableErr != nil {
			available = 0
		}
		return uc.failEvent(ctx, event, payload.Product,
			fmt.Sprintf("stock insuficiente para %s (disponible: %d)",
				payload.Product, available))
	}
	if err != nil {
		return err
	}
	if !created {
		return nil // reserva ya existente: evento duplicado
	}

	reserved, err := events.New(events.InventoryReserved, event.Saga(), event.OrderID,
		InventoryReservedPayload{
			OrderID:  event.OrderID,
			Product:  payload.Product,
			Quantity: payload.Quantity,
			Address:  payload.Address,
		})
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, events.InventoryReserved, reserved)
}
```

Este handler muestra la esencia de la coreografía: el servicio **no recibe una
orden de un orquestador**. Consume `payment.completed`, decide localmente si
puede reservar y publica `inventory.reserved` o `inventory.failed` para que los
demás reaccionen.

## 4. Compensación en cadena

### 4.1. El envío falla y publica `shipping.failed`

`services/shipping-service/internal/application/schedule_shipment.go`

```go
reason := ""
switch {
case uc.fail:
	reason = "fallo inyectado por FAIL_SHIPPING=true"
case !domain.ValidAddress(payload.Address):
	reason = "dirección inválida"
}
if reason != "" {
	if err := shipment.Fail(reason); err != nil {
		return err
	}
	if err := uc.repository.Save(ctx, shipment); err != nil {
		return err
	}
	failed, err := events.New(events.ShippingFailed, event.Saga(), event.OrderID,
		ShippingFailedPayload{OrderID: event.OrderID, Reason: reason})
	if err != nil {
		return err
	}
	return uc.publisher.Publish(ctx, events.ShippingFailed, failed)
}
```

### 4.2. Inventario libera el stock

`services/inventory-service/internal/application/release_stock.go`

```go
released, err := uc.repository.Release(ctx, event.OrderID)
if errors.Is(err, ports.ErrNotFound) {
	return nil // no hay reserva que liberar
}
if err != nil {
	return err
}
if !released {
	return nil // ya estaba liberada
}

releasedEvent, err := events.New(events.InventoryReleased, event.Saga(), event.OrderID,
	InventoryReleasedPayload{OrderID: event.OrderID, Reason: payload.Reason})
if err != nil {
	return err
}
return uc.publisher.Publish(ctx, events.InventoryReleased, releasedEvent)
```

### 4.3. Pago reembolsa y cierra la cadena

`services/payment-service/internal/application/refund_payment.go`

```go
if !payment.IsAuthorized() {
	return nil
}
if err := payment.Refund(reason); err != nil {
	return err
}
if err := uc.repository.Save(ctx, payment); err != nil {
	return err
}

refunded, err := events.New(events.PaymentRefunded, event.Saga(), event.OrderID,
	PaymentRefundedPayload{
		OrderID: payment.OrderID,
		Reason:  reason,
		Amount:  payment.Amount,
	})
if err != nil {
	return err
}
return uc.publisher.Publish(ctx, events.PaymentRefunded, refunded)
```

La cadena completa es:

```text
shipping.failed
  → inventory.released
  → payment.refunded
  → order CANCELLED
```

Cada paso es idempotente: si no hay reserva o el pago no está autorizado, el
handler termina sin publicar y sin romper la saga.

## 5. Máquina de estados del pedido

`services/order-service/internal/domain/order.go`

```go
func (o *Order) MarkCompleted() error {
	switch o.Status {
	case StatusCompleted:
		return nil
	case StatusInventoryReserved:
		o.setStatus(StatusCompleted)
		return nil
	default:
		return o.transitionError(StatusCompleted)
	}
}

func (o *Order) Cancel(reason string) error {
	switch o.Status {
	case StatusCancelled:
		if reason != "" {
			o.Reason = reason
			o.UpdatedAt = time.Now().UTC()
		}
		return nil
	case StatusCompleted:
		return o.transitionError(StatusCancelled)
	default:
		o.Status = StatusCancelled
		o.Reason = reason
		o.UpdatedAt = time.Now().UTC()
		return nil
	}
}
```

Las transiciones viven en el dominio, no en los handlers. Esto permite probar
la lógica sin RabbitMQ ni PostgreSQL y evita estados imposibles, como cancelar
un pedido ya completado.

## 6. Adaptador que implementa un puerto

`services/order-service/internal/ports/ports.go`

```go
type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
	Update(ctx context.Context, order *domain.Order) error
	Get(ctx context.Context, id string) (*domain.Order, error)
	List(ctx context.Context) ([]domain.Order, error)
}

type EventPublisher interface {
	Publish(ctx context.Context, routingKey string, event events.Event) error
}
```

`services/order-service/internal/adapters/messaging/messaging.go`

```go
type Publisher struct {
	inner *broker.Publisher
}

func NewPublisher(config PublisherConfig) (*Publisher, error) {
	inner, err := broker.NewPublisher(config.URL, config.Exchange, config.Log, config.Metrics)
	if err != nil {
		return nil, err
	}
	return &Publisher{inner: inner}, nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, event events.Event) error {
	return p.inner.Publish(ctx, routingKey, event)
}
```

La capa de aplicación depende de `ports.EventPublisher`; el adaptador de
mensajería conecta esa interfaz con RabbitMQ. Lo mismo ocurre con
`ports.OrderRepository` y el adaptador de persistencia PostgreSQL. Esto permite
usar fakes en las pruebas unitarias y cambiar la infraestructura sin tocar los
casos de uso.
