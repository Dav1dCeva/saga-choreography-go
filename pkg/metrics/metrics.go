// Package metrics registra los contadores Prometheus de cada servicio.
package metrics

import "github.com/prometheus/client_golang/prometheus"

// Counters agrupa las métricas exigidas por la rúbrica.
type Counters struct {
	Published        prometheus.Counter
	Consumed         prometheus.Counter
	Failed           prometheus.Counter
	SagasStarted     prometheus.Counter
	SagasCompleted   prometheus.Counter
	SagasCompensated prometheus.Counter
}

// New crea y registra los contadores con la etiqueta service.
func New(service string, registerer prometheus.Registerer) *Counters {
	labels := prometheus.Labels{"service": service}
	counter := func(name, help string) prometheus.Counter {
		return prometheus.NewCounter(prometheus.CounterOpts{
			Name:        name,
			Help:        help,
			ConstLabels: labels,
		})
	}

	counters := &Counters{
		Published:        counter("saga_events_published_total", "Eventos publicados por el servicio."),
		Consumed:         counter("saga_events_consumed_total", "Eventos consumidos por el servicio."),
		Failed:           counter("saga_events_failed_total", "Eventos que terminaron en error o DLQ."),
		SagasStarted:     counter("saga_sagas_started_total", "Sagas iniciadas."),
		SagasCompleted:   counter("saga_sagas_completed_total", "Sagas completadas."),
		SagasCompensated: counter("saga_sagas_compensated_total", "Sagas compensadas o canceladas."),
	}
	registerer.MustRegister(
		counters.Published,
		counters.Consumed,
		counters.Failed,
		counters.SagasStarted,
		counters.SagasCompleted,
		counters.SagasCompensated,
	)
	return counters
}

// EventPublished incrementa el contador de eventos publicados.
func (c *Counters) EventPublished() { c.Published.Inc() }

// EventConsumed incrementa el contador de eventos consumidos.
func (c *Counters) EventConsumed() { c.Consumed.Inc() }

// EventFailed incrementa el contador de eventos fallidos.
func (c *Counters) EventFailed() { c.Failed.Inc() }

// SagaStarted incrementa el contador de sagas iniciadas.
func (c *Counters) SagaStarted() { c.SagasStarted.Inc() }

// SagaCompleted incrementa el contador de sagas completadas.
func (c *Counters) SagaCompleted() { c.SagasCompleted.Inc() }

// SagaCompensated incrementa el contador de sagas compensadas.
func (c *Counters) SagaCompensated() { c.SagasCompensated.Inc() }
