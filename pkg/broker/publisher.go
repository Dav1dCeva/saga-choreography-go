// Package broker implementa los adaptadores de mensajería con RabbitMQ.
package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"saga-choreography-go/pkg/events"
)

// PublisherMetrics permite instrumentar las publicaciones.
type PublisherMetrics interface {
	EventPublished()
}

// Publisher publica eventos persistentes en un exchange topic.
type Publisher struct {
	conn     *amqp.Connection
	channel  *amqp.Channel
	exchange string
	log      *slog.Logger
	metrics  PublisherMetrics
	mu       sync.Mutex
}

// NewPublisher conecta a RabbitMQ con reintentos y declara el exchange.
func NewPublisher(url, exchange string, log *slog.Logger, metrics PublisherMetrics) (*Publisher, error) {
	var (
		conn *amqp.Connection
		err  error
	)
	for attempt := 1; attempt <= 30; attempt++ {
		conn, err = amqp.DialConfig(url, amqp.Config{
			Heartbeat: 10 * time.Second,
			Locale:    "en_US",
		})
		if err == nil {
			break
		}
		if log != nil {
			log.Warn("esperando a RabbitMQ", "intento", attempt, "error", err)
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("conectar a RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("abrir canal: %w", err)
	}
	if err := channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("declarar exchange %s: %w", exchange, err)
	}

	return &Publisher{
		conn:     conn,
		channel:  channel,
		exchange: exchange,
		log:      log,
		metrics:  metrics,
	}, nil
}

// Publish serializa y publica el sobre del evento de forma persistente.
func (p *Publisher) Publish(ctx context.Context, routingKey string, event events.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("serializar evento: %w", err)
	}

	p.mu.Lock()
	err = p.channel.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    event.EventID,
		Timestamp:    event.Timestamp,
		Type:         event.Type,
		Body:         body,
	})
	p.mu.Unlock()
	if err != nil {
		return fmt.Errorf("publicar %s: %w", routingKey, err)
	}

	if p.metrics != nil {
		p.metrics.EventPublished()
	}
	if p.log != nil {
		p.log.Info("evento publicado",
			"saga_id", event.SagaID,
			"order_id", event.OrderID,
			"event_id", event.EventID,
			"type", event.Type,
			"routing_key", routingKey,
		)
	}
	return nil
}

// Close cierra el canal y la conexión.
func (p *Publisher) Close() error {
	_ = p.channel.Close()
	return p.conn.Close()
}
