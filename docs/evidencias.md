# Guía de evidencias

Esta guía indica **qué ejecutar y qué capturar** para demostrar el funcionamiento
del proyecto. Las capturas se pueden guardar en `docs/img/` (crear la carpeta si
se desea) y enlazarlas desde el informe o el README.

> Si se usa Podman, reemplazar `docker compose` por `podman-compose` en los
> comandos de esta guía. En Windows, usar `scripts\saga.cmd` y
> `scripts\e2e.cmd`; ver [`windows.md`](windows.md).

## 1. Preparación del entorno

```bash
cp .env.example .env
make up
make ps
```

Capturar:

- Terminal con `make up` finalizado sin errores.
- Salida de `make ps` con los 8 contenedores (`rabbitmq`, `postgres`, los 4
  servicios, `prometheus` y `grafana`) en estado `healthy`/`Up`.
- Healthchecks:

```bash
for port in 8081 8082 8083 8084; do
  echo -n "$port: "
  curl -s "http://localhost:$port/health"
  echo
done
```

Evidencia sugerida: `docs/img/01-stack-healthy.png`.

## 2. RabbitMQ Management

Abrir http://localhost:15672 con `guest` / `guest`.

Capturar:

- Pestaña **Queues and Streams** con:
  - `order-service.queue`
  - `payment-service.queue`
  - `inventory-service.queue`
  - `shipping-service.queue`
  - las cuatro `<cola>.dlq`
- Pestaña **Exchanges** con `saga.events` (topic) y `saga.dlx` (fanout).
- Detalle de `order-service.queue` → **Bindings**, mostrando las routing keys:
  `payment.completed`, `inventory.reserved`, `shipping.scheduled`,
  `payment.failed`, `payment.refunded`, `inventory.failed`, `shipping.failed`.
- Estado de las colas después de un escenario: consumidores conectados y
  mensajes en 0 (o el mensaje en la DLQ si se provocó un fallo técnico).

Evidencia sugerida: `docs/img/02-rabbitmq-queues.png`,
`docs/img/03-rabbitmq-bindings.png`.

## 3. Escenario exitoso

```bash
make demo-ok
```

La salida esperada termina con:

```text
OK: <order_id> terminó en COMPLETED
```

Capturar:

- Salida del script con las transiciones `PENDING → PAYMENT_COMPLETED →
  INVENTORY_RESERVED → COMPLETED`.
- JSON final con `"status":"COMPLETED"` y `"tracking":"TRK-<order_id>"`.
- Logs correlacionados:

```bash
ORDER_ID=<order_id>
docker compose logs --no-color order-service payment-service inventory-service shipping-service \
  | grep "$ORDER_ID"
```

Con Podman:

```bash
podman-compose logs --no-color order-service payment-service inventory-service shipping-service \
  | grep "$ORDER_ID"
```

Se deben ver las acciones `pago autorizado`, `stock reservado`, `envío
programado` y `saga completada`, todas con el mismo `saga_id`.

Evidencia sugerida: `docs/img/04-demo-ok.png`,
`docs/img/05-logs-ok.png`.

## 4. Fallo de pago

```bash
make demo-payment-fail
```

La salida esperada:

```text
OK: <order_id> terminó en CANCELLED
```

Capturar:

- JSON final con `"status":"CANCELLED"` y
  `"reason":"el monto supera el límite de 1000.00"`.
- Log de `payment.failed` y la cancelación en `order-service`.
- En RabbitMQ: no deben quedar mensajes pendientes en las colas de la saga.

Evidencia sugerida: `docs/img/06-demo-payment-fail.png`.

## 5. Fallo de inventario

```bash
make demo-inventory-fail
```

La salida esperada:

```text
OK: <order_id> terminó en CANCELLED
```

Capturar:

- JSON final con motivo de stock insuficiente, por ejemplo:
  `"stock insuficiente para laptop (disponible: 3)"`.
- Logs que muestren la secuencia:
  `payment.completed → inventory.failed → payment.refunded → order CANCELLED`.
- En la base `payments`, el pago debe quedar reembolsado; en `orders`, el pedido
  `CANCELLED`.

Consulta opcional:

```bash
docker compose exec -T postgres psql -U saga -d payments \
  -c "SELECT order_id, status, reason FROM payments ORDER BY created_at DESC LIMIT 5;"
```

Evidencia sugerida: `docs/img/07-demo-inventory-fail.png`.

## 6. Fallo de envío y compensación en cadena

```bash
make demo-shipping-fail
```

La salida esperada:

```text
OK: <order_id> terminó en CANCELLED
```

Capturar:

- JSON final con `"status":"CANCELLED"` y `"reason":"dirección inválida"`.
- Logs que demuestren la cadena completa:

```text
shipping.failed
  → inventory.released
  → payment.refunded
  → order CANCELLED
```

Comando recomendado:

```bash
ORDER_ID=<order_id>
docker compose logs --no-color | grep "$ORDER_ID" | grep -E \
  'shipping.failed|inventory.released|payment.refunded|stock liberado|pago reembolsado|saga compensada'
```

- En PostgreSQL: reserva liberada, pago reembolsado y pedido cancelado.

Evidencia sugerida: `docs/img/08-demo-shipping-fail.png`,
`docs/img/09-logs-compensacion.png`.

## 7. Dashboard de Grafana

Abrir http://localhost:3000 con `admin` / `admin`.

Capturar el dashboard **SAGA por Coreografía** con:

- Eventos publicados por servicio.
- Eventos consumidos.
- Sagas iniciadas.
- Sagas completadas.
- Sagas compensadas.
- Fallos de procesamiento.

Después de ejecutar los 4 escenarios, los contadores de completadas y
compensadas deben haber aumentado. Si el panel no muestra datos, verificar
**Connections → Data sources → Prometheus** apuntando a
`http://prometheus:9090`.

Evidencia sugerida: `docs/img/10-grafana.png`.

## 8. Prometheus

Abrir http://localhost:9090 y ejecutar consultas como:

```promql
saga_sagas_started_total
saga_sagas_completed_total
saga_sagas_compensated_total
saga_events_published_total
saga_events_consumed_total
saga_events_failed_total
```

Capturar:

- Pestaña **Status → Targets** con los 4 servicios en `UP`.
- Una consulta con resultados por servicio.

Evidencia sugerida: `docs/img/11-prometheus-targets.png`,
`docs/img/12-prometheus-query.png`.

## 9. Pruebas y cobertura

```bash
make test
make cover-core
go tool cover -func=coverage-core.out | tail -n 1
make integration
```

Valores de referencia obtenidos en este entorno:

| Paquete | Cobertura |
|---|---|
| order application | 79.3% |
| payment application | 79.4% |
| inventory application | 85.7% |
| shipping application | 82.4% |
| dominios | 98–100% |
| **domain + application** | **87.3%** |
| total del repositorio | 33.0% |

Capturar:

- Salida de `make test` con todos los paquetes `ok`.
- Salida de `make cover-core` con `total: 87.3%`.
- Salida de `make integration` con `ok` en `pkg/broker` y
  `services/order-service/internal/adapters/persistence`.

Evidencia sugerida: `docs/img/13-tests.png`,
`docs/img/14-coverage.png`, `docs/img/15-integration.png`.

## 10. Pipeline de GitHub Actions

Hacer push a un repositorio de GitHub y abrir la pestaña **Actions**.

Capturar:

- Job `quality` en verde: `go vet`, `golangci-lint`, pruebas con `-race` y
  cobertura.
- Los 4 jobs de `build-images` en verde.
- Artefacto `coverage` descargable.

Evidencia sugerida: `docs/img/16-ci.png`.

## 11. Limpieza entre escenarios

Para repetir la demo sin reiniciar contenedores:

```bash
make reset
```

Esto restaura el stock (`laptop=3`, `phone=10`, `headphones=20`, `monitor=4`),
vacía reservas y trunca pedidos, pagos y envíos.

Para detener todo:

```bash
make down
```

## Checklist rápido de evidencias

| Evidencia | Comando / lugar | Archivo sugerido |
|---|---|---|
| Stack healthy | `make ps` | `01-stack-healthy.png` |
| Colas y DLQ | RabbitMQ UI | `02-rabbitmq-queues.png` |
| Bindings | RabbitMQ UI | `03-rabbitmq-bindings.png` |
| Éxito | `make demo-ok` | `04-demo-ok.png` |
| Logs éxito | `docker compose logs` | `05-logs-ok.png` |
| Fallo pago | `make demo-payment-fail` | `06-demo-payment-fail.png` |
| Fallo inventario | `make demo-inventory-fail` | `07-demo-inventory-fail.png` |
| Fallo envío | `make demo-shipping-fail` | `08-demo-shipping-fail.png` |
| Compensación | logs filtrados | `09-logs-compensacion.png` |
| Grafana | http://localhost:3000 | `10-grafana.png` |
| Prometheus | http://localhost:9090 | `11-prometheus-targets.png` |
| Cobertura | `make cover-core` | `14-coverage.png` |
| Integración | `make integration` | `15-integration.png` |
| CI | GitHub Actions | `16-ci.png` |
