package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"saga-choreography-go/pkg/events"
)

// IdempotencyStore abstrae el almacén de event_id procesados.
type IdempotencyStore interface {
	MarkProcessed(ctx context.Context, eventID, eventType string) (bool, error)
	UnmarkProcessed(ctx context.Context, eventID string) error
}

// ConsumerMetrics instrumenta el consumo de eventos.
type ConsumerMetrics interface {
	EventConsumed()
	EventFailed()
}

// Handler procesa un evento; si devuelve error se reintenta y luego va a DLQ.
type Handler func(ctx context.Context, event events.Event) error

// ConsumerConfig reúne la topología de una cola de servicio.
type ConsumerConfig struct {
	URL          string
	Exchange     string
	Queue        string
	RoutingKeys  []string
	DeadLetterEx string
	Handler      Handler
	Idempotency  IdempotencyStore
	Log          *slog.Logger
	Metrics      ConsumerMetrics
	MaxRetries   int
}

// Consumer consume una cola durable con ack manual y DLQ.
type Consumer struct {
	cfg     ConsumerConfig
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewConsumer conecta, declara exchange, cola durable, DLQ y bindings.
func NewConsumer(cfg ConsumerConfig) (*Consumer, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.DeadLetterEx == "" {
		cfg.DeadLetterEx = "saga.dlx"
	}

	conn, err := amqp.DialConfig(cfg.URL, amqp.Config{
		Heartbeat: 10 * time.Second,
		Locale:    "en_US",
	})
	if err != nil {
		return nil, fmt.Errorf("conectar consumidor a RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("abrir canal consumidor: %w", err)
	}

	if err := declareTopology(channel, cfg); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, err
	}
	if err := channel.Qos(1, 0, false); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("configurar prefetch: %w", err)
	}

	return &Consumer{cfg: cfg, conn: conn, channel: channel}, nil
}

func declareTopology(channel *amqp.Channel, cfg ConsumerConfig) error {
	if err := channel.ExchangeDeclare(cfg.Exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarar exchange %s: %w", cfg.Exchange, err)
	}
	if err := channel.ExchangeDeclare(cfg.DeadLetterEx, "fanout", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarar DLX %s: %w", cfg.DeadLetterEx, err)
	}

	dlq, err := channel.QueueDeclare(cfg.Queue+".dlq", true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declarar DLQ %s: %w", cfg.Queue+".dlq", err)
	}
	if err := channel.QueueBind(dlq.Name, "", cfg.DeadLetterEx, false, nil); err != nil {
		return fmt.Errorf("enlazar DLQ: %w", err)
	}

	queue, err := channel.QueueDeclare(cfg.Queue, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    cfg.DeadLetterEx,
		"x-dead-letter-routing-key": cfg.Queue,
	})
	if err != nil {
		return fmt.Errorf("declarar cola %s: %w", cfg.Queue, err)
	}
	for _, routingKey := range cfg.RoutingKeys {
		if err := channel.QueueBind(queue.Name, routingKey, cfg.Exchange, false, nil); err != nil {
			return fmt.Errorf("enlazar %s a %s: %w", routingKey, cfg.Queue, err)
		}
	}
	return nil
}

// Run consume hasta que el contexto se cancele o el canal se cierre.
func (c *Consumer) Run(ctx context.Context) error {
	deliveries, err := c.channel.Consume(c.cfg.Queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumir %s: %w", c.cfg.Queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}
			c.handle(ctx, delivery)
		}
	}
}

func (c *Consumer) handle(ctx context.Context, delivery amqp.Delivery) {
	var event events.Event
	if err := json.Unmarshal(delivery.Body, &event); err != nil {
		c.logf("error", "mensaje ilegible", "error", err)
		c.fail()
		_ = delivery.Nack(false, false)
		return
	}
	if err := event.Validate(); err != nil {
		c.logf("error", "evento inválido", "error", err)
		c.fail()
		_ = delivery.Nack(false, false)
		return
	}

	c.logf("info", "evento recibido",
		"saga_id", event.SagaID,
		"order_id", event.OrderID,
		"event_id", event.EventID,
		"type", event.Type,
		"queue", c.cfg.Queue,
	)
	if c.cfg.Metrics != nil {
		c.cfg.Metrics.EventConsumed()
	}

	if c.cfg.Idempotency != nil {
		isNew, err := c.cfg.Idempotency.MarkProcessed(ctx, event.EventID, event.Type)
		if err != nil {
			c.logf("error", "no se pudo verificar idempotencia", "error", err)
			c.fail()
			_ = delivery.Nack(false, true)
			return
		}
		if !isNew {
			c.logf("info", "evento duplicado ignorado", "event_id", event.EventID, "type", event.Type)
			_ = delivery.Ack(false)
			return
		}
	}

	handlerCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	err := c.retry(handlerCtx, event)
	cancel()
	if err != nil {
		c.logf("error", "procesamiento fallido; enviando a DLQ",
			"event_id", event.EventID,
			"type", event.Type,
			"error", err,
		)
		c.fail()
		if c.cfg.Idempotency != nil {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = c.cfg.Idempotency.UnmarkProcessed(cleanupCtx, event.EventID)
			cleanupCancel()
		}
		_ = delivery.Nack(false, false)
		return
	}

	if err := delivery.Ack(false); err != nil {
		c.logf("error", "no se pudo confirmar el evento", "error", err)
	}
}

func (c *Consumer) retry(ctx context.Context, event events.Event) error {
	var lastErr error
	for attempt := 1; attempt <= c.cfg.MaxRetries; attempt++ {
		if err := c.cfg.Handler(ctx, event); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < c.cfg.MaxRetries {
			backoff := time.Duration(attempt) * 250 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return lastErr
}

func (c *Consumer) fail() {
	if c.cfg.Metrics != nil {
		c.cfg.Metrics.EventFailed()
	}
}

func (c *Consumer) logf(level, message string, args ...any) {
	if c.cfg.Log == nil {
		return
	}
	if level == "error" {
		c.cfg.Log.Error(message, args...)
		return
	}
	c.cfg.Log.Info(message, args...)
}

// Close cierra el canal y la conexión del consumidor.
func (c *Consumer) Close() error {
	_ = c.channel.Close()
	return c.conn.Close()
}
