//go:build integration

package broker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"saga-choreography-go/pkg/events"
)

func TestPublisherConsumerIntegration(t *testing.T) {
	url := os.Getenv("TEST_RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	suffix := uuid.NewString()
	exchange := "saga.test." + suffix
	queue := "saga.test.queue." + suffix
	routingKey := "test.event"

	publisher, err := NewPublisher(url, exchange, nil, nil)
	if err != nil {
		t.Fatalf("crear publicador: %v", err)
	}
	defer func() { _ = publisher.Close() }()

	received := make(chan events.Event, 1)
	consumer, err := NewConsumer(ConsumerConfig{
		URL:          url,
		Exchange:     exchange,
		Queue:        queue,
		RoutingKeys:  []string{routingKey},
		DeadLetterEx: "saga.test.dlx." + suffix,
		MaxRetries:   1,
		Handler: func(_ context.Context, event events.Event) error {
			received <- event
			return nil
		},
	})
	if err != nil {
		t.Fatalf("crear consumidor: %v", err)
	}
	defer func() { _ = consumer.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = consumer.Run(ctx) }()
	time.Sleep(500 * time.Millisecond)

	event, err := events.New(routingKey, "saga-integration", "order-integration", map[string]string{"ok": "true"})
	if err != nil {
		t.Fatalf("crear evento: %v", err)
	}
	if err := publisher.Publish(ctx, routingKey, event); err != nil {
		t.Fatalf("publicar: %v", err)
	}

	select {
	case got := <-received:
		if got.EventID != event.EventID || got.OrderID != event.OrderID {
			t.Fatalf("evento distinto: %+v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout esperando el evento")
	}

	cancel()
	cleanupBroker(t, url, queue, exchange, suffix)
}

func cleanupBroker(t *testing.T, url, queue, exchange, suffix string) {
	t.Helper()
	conn, err := amqp.Dial(url)
	if err != nil {
		t.Logf("limpieza: no se pudo conectar: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()
	channel, err := conn.Channel()
	if err != nil {
		t.Logf("limpieza: no se pudo abrir canal: %v", err)
		return
	}
	defer func() { _ = channel.Close() }()
	_, _ = channel.QueueDelete(queue, false, false, false)
	_, _ = channel.QueueDelete(queue+".dlq", false, false, false)
	_ = channel.ExchangeDelete(exchange, false, false)
	_ = channel.ExchangeDelete("saga.test.dlx."+suffix, false, false)
}
