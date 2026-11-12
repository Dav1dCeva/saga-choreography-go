// Package messaging adapta RabbitMQ a los puertos del servicio.
package messaging

import (
	"context"
	"log/slog"

	"saga-choreography-go/pkg/broker"
	"saga-choreography-go/pkg/events"
)

// PublisherConfig agrupa la configuración del publicador.
type PublisherConfig struct {
	URL      string
	Exchange string
	Log      *slog.Logger
	Metrics  broker.PublisherMetrics
}

// Publisher implementa ports.EventPublisher sobre RabbitMQ.
type Publisher struct {
	inner *broker.Publisher
}

// NewPublisher construye el adaptador de publicación.
func NewPublisher(config PublisherConfig) (*Publisher, error) {
	inner, err := broker.NewPublisher(config.URL, config.Exchange, config.Log, config.Metrics)
	if err != nil {
		return nil, err
	}
	return &Publisher{inner: inner}, nil
}

// Publish publica un evento en el exchange configurado.
func (p *Publisher) Publish(ctx context.Context, routingKey string, event events.Event) error {
	return p.inner.Publish(ctx, routingKey, event)
}

// Close cierra la conexión con RabbitMQ.
func (p *Publisher) Close() error {
	return p.inner.Close()
}

// ConsumerConfig es la topología que necesita el consumidor del servicio.
type ConsumerConfig = broker.ConsumerConfig

// Consumer implementa el bucle de consumo con ack manual y DLQ.
type Consumer struct {
	inner *broker.Consumer
}

// NewConsumer construye el adaptador de consumo.
func NewConsumer(config ConsumerConfig) (*Consumer, error) {
	inner, err := broker.NewConsumer(config)
	if err != nil {
		return nil, err
	}
	return &Consumer{inner: inner}, nil
}

// Run consume mensajes hasta que el contexto se cancele.
func (c *Consumer) Run(ctx context.Context) error {
	return c.inner.Run(ctx)
}

// Close cierra el canal y la conexión del consumidor.
func (c *Consumer) Close() error {
	return c.inner.Close()
}
