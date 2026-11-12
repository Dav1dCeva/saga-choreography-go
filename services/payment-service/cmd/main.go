// payment-service cobra y reembolsa pedidos.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"saga-choreography-go/pkg/config"
	"saga-choreography-go/pkg/events"
	"saga-choreography-go/pkg/httpx"
	"saga-choreography-go/pkg/metrics"
	"saga-choreography-go/pkg/postgres"
	"saga-choreography-go/services/payment-service/internal/adapters/messaging"
	"saga-choreography-go/services/payment-service/internal/adapters/persistence"
	"saga-choreography-go/services/payment-service/internal/application"
)

const serviceName = "payment-service"

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).With("service", serviceName)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.Open(ctx, config.String("DATABASE_URL", "postgres://saga:saga@localhost:5433/payments?sslmode=disable"))
	if err != nil {
		log.Error("conectar postgres", "error", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	if err := persistence.Migrate(ctx, db); err != nil {
		log.Error("migrar esquema", "error", err)
		os.Exit(1)
	}
	idempotency := postgres.NewIdempotencyStore(db)
	if err := idempotency.EnsureTable(ctx); err != nil {
		log.Error("crear tabla de idempotencia", "error", err)
		os.Exit(1)
	}

	registry := prometheus.NewRegistry()
	counters := metrics.New(serviceName, registry)

	publisher, err := messaging.NewPublisher(messaging.PublisherConfig{
		URL:      config.String("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Exchange: config.String("RABBITMQ_EXCHANGE", "saga.events"),
		Log:      log,
		Metrics:  counters,
	})
	if err != nil {
		log.Error("conectar RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer func() { _ = publisher.Close() }()

	repository := persistence.NewPaymentRepository(db)
	chargeOrder := application.NewChargeOrder(repository, publisher, application.ChargeConfig{
		FailPayment:     config.Bool("FAIL_PAYMENT", false),
		MaxAmount:       config.Float("MAX_PAYMENT_AMOUNT", 1000),
		BlockedCustomer: config.String("BLOCKED_CUSTOMER", "blocked"),
	}, log)
	refundPayment := application.NewRefundPayment(repository, publisher, log)

	handler := func(ctx context.Context, event events.Event) error {
		switch event.Type {
		case events.OrderCreated:
			return chargeOrder.Handle(ctx, event)
		case events.InventoryFailed:
			return refundPayment.Handle(ctx, event, "stock no disponible")
		case events.InventoryReleased:
			return refundPayment.Handle(ctx, event, "envío fallido")
		default:
			return errors.New("evento no soportado: " + event.Type)
		}
	}

	consumer, err := messaging.NewConsumer(messaging.ConsumerConfig{
		URL:      config.String("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		Exchange: config.String("RABBITMQ_EXCHANGE", "saga.events"),
		Queue:    "payment-service.queue",
		RoutingKeys: []string{
			events.OrderCreated,
			events.InventoryFailed,
			events.InventoryReleased,
		},
		Handler:     handler,
		Idempotency: idempotency,
		Log:         log,
		Metrics:     counters,
	})
	if err != nil {
		log.Error("crear consumidor", "error", err)
		os.Exit(1)
	}
	defer func() { _ = consumer.Close() }()

	go func() {
		if err := consumer.Run(ctx); err != nil && ctx.Err() == nil {
			log.Error("consumidor detenido", "error", err)
			stop()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health(db))
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:              config.String("HTTP_ADDR", ":8082"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("HTTP escuchando", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("servidor HTTP", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("apagando servicio")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Warn("apagado HTTP", "error", err)
	}
}
