# Cómo ejecutar y probar el proyecto

Guía paso a paso para levantar la saga, ejecutar los 4 escenarios y ver la traza
legible de cada pedido.

---

## 1. Qué necesitas

### Windows

- Docker Desktop abierto.
- Docker Desktop usando **contenedores Linux**.
- PowerShell 5.1 o superior.
- **No hace falta** instalar `make`, Bash ni Python para ejecutar la demo.
- Go solo es necesario si quieres correr las pruebas unitarias.

### Linux / macOS

- Podman con `podman-compose`, o Docker con Compose.
- `make`, `curl` y `python3`.

---

## 2. Ejecutar en Windows

Abrir PowerShell dentro de la carpeta descomprimida:

```powershell
cd C:\ruta\saga-choreography-go
```

Crear configuración:

```powershell
Copy-Item .env.example .env
```

Opcional: limpiar una ejecución anterior.

```powershell
.\scripts\saga.cmd down
```

Levantar todo:

```powershell
.\scripts\saga.cmd up
```

Comprobar contenedores:

```powershell
.\scripts\saga.cmd ps
```

Deben aparecer 8 contenedores, con RabbitMQ, PostgreSQL y los 4 servicios en
estado `healthy`.

Comprobar healthchecks:

```powershell
Invoke-RestMethod http://localhost:8081/health
Invoke-RestMethod http://localhost:8082/health
Invoke-RestMethod http://localhost:8083/health
Invoke-RestMethod http://localhost:8084/health
```

Todos deben responder:

```text
status
------
ok
```

---

## 3. Ejecutar en Linux / macOS

```bash
cd /ruta/saga-choreography-go
cp .env.example .env
```

Opcional: limpiar una ejecución anterior.

```bash
make down
```

Levantar:

```bash
make up
```

Comprobar:

```bash
make ps
```

Healthchecks:

```bash
for port in 8081 8082 8083 8084; do
  echo -n "$port: "
  curl -s "http://localhost:$port/health"
  echo
done
```

---

## 4. Ejecutar los 4 escenarios

### Windows

```powershell
.\scripts\saga.cmd demo-ok
.\scripts\saga.cmd demo-payment-fail
.\scripts\saga.cmd demo-inventory-fail
.\scripts\saga.cmd demo-shipping-fail
```

### Linux / macOS

```bash
make demo-ok
make demo-payment-fail
make demo-inventory-fail
make demo-shipping-fail
```

Resultados esperados:

| Escenario | Resultado |
|---|---|
| `demo-ok` | `COMPLETED` con `tracking` `TRK-<order_id>` |
| `demo-payment-fail` | `CANCELLED` por superar el límite de pago |
| `demo-inventory-fail` | `CANCELLED` por stock insuficiente |
| `demo-shipping-fail` | `CANCELLED` por dirección inválida |

Cada script imprime al final algo como:

```text
OK: <order_id> terminó en COMPLETED
```

Guarda ese `order_id` para ver la traza.

---

## 5. Ver la traza legible de una saga

El script `trace` convierte los logs JSON en una línea de tiempo ordenada.
Muestra eventos recibidos, eventos publicados, acciones ejecutadas y el final
de la saga.

### Windows

Último pedido creado:

```powershell
.\scripts\trace.cmd
```

Un pedido concreto:

```powershell
.\scripts\trace.cmd <order_id>
```

También se puede usar el helper general:

```powershell
.\scripts\saga.cmd trace
.\scripts\saga.cmd trace <order_id>
```

### Linux / macOS

Último pedido creado:

```bash
make trace
```

Un pedido concreto:

```bash
./scripts/trace.sh <order_id>
```

O:

```bash
ORDER_ID=<order_id> make trace
```

### Ejemplo de salida

```text
SAGA 5b0e...

HORA         SERVICIO   DETALLE
------------------------------------------------------------------------------
01:20:10.123 order      <- recibe    order.created
01:20:10.125 order      -> publica   order.created
01:20:10.180 payment    <- recibe    order.created
01:20:10.190 payment    ** ACCIÓN    pago autorizado
01:20:10.195 payment    -> publica   payment.completed
01:20:10.230 inventory  <- recibe    payment.completed
01:20:10.240 inventory  ** ACCIÓN    stock reservado  (laptop)
01:20:10.245 inventory  -> publica   inventory.reserved
01:20:10.280 shipping   <- recibe    inventory.reserved
01:20:10.290 shipping   ** ACCIÓN    envío programado  (TRK-...)
01:20:10.295 shipping   -> publica   shipping.scheduled
01:20:10.330 order      <- recibe    shipping.scheduled
01:20:10.335 order      ** ACCIÓN    shipping.scheduled  => estado COMPLETED
01:20:10.340 order      OK SAGA COMPLETADA  tracking TRK-...
```

Cómo leerlo:

- `<-` el servicio recibió un evento.
- `->` el servicio publicó un evento.
- `**` el servicio ejecutó una acción de negocio.
- `OK` la saga terminó completada.
- `XX` la saga terminó cancelada.

Para el escenario `shipping-fail`, la traza debe mostrar:

```text
shipping   -> publica   shipping.failed
inventory  <- recibe    shipping.failed
inventory  ** ACCIÓN    stock liberado
inventory  -> publica   inventory.released
payment    <- recibe    inventory.released
payment    ** ACCIÓN    pago reembolsado
payment    -> publica   payment.refunded
order      <- recibe    payment.refunded
order      ** ACCIÓN    payment.refunded  => estado CANCELLED
```

---

## 6. Abrir la interfaz web

| Recurso | URL | Credenciales |
|---|---|---|
| API de pedidos | http://localhost:8081/orders | — |
| RabbitMQ | http://localhost:15672 | `guest` / `guest` |
| Prometheus | http://localhost:9090 | — |
| Grafana | http://localhost:3000 | `admin` / `admin` |

En Grafana buscar el dashboard:

```text
SAGA por Coreografía
```

En Prometheus revisar:

```text
Status → Targets
```

Los 4 servicios deben estar `UP`.

---

## 7. Repetir demos y limpiar datos

Puedes repetir los escenarios. Ten en cuenta que `demo-ok` consume 1 unidad de
`phone`; después de varios éxitos el stock se agota.

Restaurar stock y limpiar tablas:

### Windows

```powershell
.\scripts\saga.cmd reset
```

### Linux / macOS

```bash
make reset
```

Apagar todo:

### Windows

```powershell
.\scripts\saga.cmd down
```

### Linux / macOS

```bash
make down
```

`down` elimina contenedores y volúmenes. La siguiente ejecución vuelve a crear
bases, colas y stock inicial.

---

## 8. Pruebas opcionales

Solo si tienes Go instalado.

### Windows

```powershell
.\scripts\saga.cmd test
.\scripts\saga.cmd cover-core
.\scripts\saga.cmd integration
```

### Linux / macOS

```bash
make test
make cover-core
make integration
```

`integration` requiere que el stack esté levantado.

Resultados de referencia:

- Unitarias: todos los paquetes `ok`.
- Cobertura `domain` + `application`: `87.3%`.
- Integración: `pkg/broker` y persistencia de pedidos `ok`.

---

## 9. Problemas comunes

### `docker` no se reconoce en Windows

Abrir Docker Desktop y esperar a que inicie. Comprobar:

```powershell
docker version
docker compose version
```

### PowerShell bloquea los scripts

Usar los wrappers `.cmd`, que ya incluyen `-ExecutionPolicy Bypass`:

```powershell
.\scripts\saga.cmd up
.\scripts\trace.cmd
```

O ejecutar:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\saga.ps1 up
```

### Un puerto está ocupado

```powershell
netstat -ano | findstr :8081
```

Cerrar el proceso o cambiar el puerto en `docker-compose.yml`.

### RabbitMQ no queda healthy

```powershell
docker compose -f docker-compose.yml -f docker-compose.windows.yml logs rabbitmq
```

### La traza dice “Sin eventos para ese pedido”

Verificar que el `order_id` sea el correcto y que los servicios hayan procesado
el pedido. Probar sin ID para tomar el último pedido:

```powershell
.\scripts\trace.cmd
```

```bash
./scripts/trace.sh
```

---

## 10. Checklist rápido

- [ ] Docker Desktop / Podman funcionando.
- [ ] `saga.cmd up` o `make up` termina sin errores.
- [ ] Los 8 contenedores están arriba.
- [ ] Los 4 `/health` responden `ok`.
- [ ] Los 4 escenarios terminan en el estado esperado.
- [ ] `trace.cmd` o `trace.sh` muestra la línea de tiempo.
- [ ] RabbitMQ, Prometheus y Grafana abren correctamente.
- [ ] `saga.cmd reset` o `make reset` restaura la demo.
