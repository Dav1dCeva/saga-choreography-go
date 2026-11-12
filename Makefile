SHELL := /bin/bash

# Detecta docker compose, podman-compose o el compose integrado de Podman.
COMPOSE ?= $(shell \
	if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then echo "docker compose"; \
	elif command -v podman-compose >/dev/null 2>&1; then echo "podman-compose"; \
	elif command -v podman >/dev/null 2>&1; then echo "podman compose"; \
	else echo "docker compose"; fi)

.PHONY: help up down logs ps build test vet lint cover cover-core integration trace e2e reset \
	demo-ok demo-payment-fail demo-inventory-fail demo-shipping-fail sonar sonar-down

help:
	@echo "Objetivos disponibles:"
	@echo "  make up                    levanta RabbitMQ, PostgreSQL, servicios, Prometheus y Grafana"
	@echo "  make down                  detiene y borra volúmenes"
	@echo "  make logs                  sigue los logs de todos los contenedores"
	@echo "  make test                  ejecuta pruebas unitarias"
	@echo "  make cover                 pruebas con -race y reporte de cobertura total"
	@echo "  make cover-core            cobertura solo de domain y application"
	@echo "  make integration           pruebas de integración contra el compose levantado"
	@echo "  make trace                 traza legible de la última saga (o ORDER_ID=...)"
	@echo "  make demo-ok               escenario exitoso"
	@echo "  make demo-payment-fail     fallo de pago"
	@echo "  make demo-inventory-fail   fallo de inventario"
	@echo "  make demo-shipping-fail    fallo de envío"
	@echo "  make reset                 restaura stock y limpia pedidos (sin borrar contenedores)"
	@echo "  make sonar                 levanta SonarQube opcional"

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down -v --remove-orphans

logs:
	$(COMPOSE) logs -f --tail=100

ps:
	$(COMPOSE) ps

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

cover:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -n 1

CORE_PACKAGES := $(shell go list ./services/... 2>/dev/null | grep -E '/(domain|application)$$')

cover-core:
	go test -covermode=atomic -coverprofile=coverage-core.out $(CORE_PACKAGES)
	@go tool cover -func=coverage-core.out | tail -n 1

integration:
	go test -tags=integration -count=1 ./pkg/broker ./services/order-service/internal/adapters/persistence

trace:
	./scripts/trace.sh $${ORDER_ID:-}

e2e:
	./scripts/e2e.sh $${SCENARIO:-ok}

demo-ok:
	./scripts/e2e.sh ok

demo-payment-fail:
	./scripts/e2e.sh payment-fail

demo-inventory-fail:
	./scripts/e2e.sh inventory-fail

demo-shipping-fail:
	./scripts/e2e.sh shipping-fail

reset:
	$(COMPOSE) exec -T postgres psql -U $${POSTGRES_USER:-saga} -d inventory -c \
		"UPDATE stock SET available = CASE sku WHEN 'laptop' THEN 3 WHEN 'phone' THEN 10 WHEN 'headphones' THEN 20 WHEN 'monitor' THEN 4 ELSE available END; TRUNCATE reservations;"
	$(COMPOSE) exec -T postgres psql -U $${POSTGRES_USER:-saga} -d orders -c "TRUNCATE orders;"
	$(COMPOSE) exec -T postgres psql -U $${POSTGRES_USER:-saga} -d payments -c "TRUNCATE payments;"
	$(COMPOSE) exec -T postgres psql -U $${POSTGRES_USER:-saga} -d shipping -c "TRUNCATE shipments;"
	@echo "Estado de demo restaurado."

sonar:
	$(COMPOSE) -f docker-compose.sonar.yml up -d

sonar-down:
	$(COMPOSE) -f docker-compose.sonar.yml down -v
