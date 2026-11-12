# Ejecutar en Windows con Docker Desktop

Esta guía permite abrir y ejecutar el proyecto en Windows **sin `make`, sin
Bash y sin Python**. Solo se necesita Docker Desktop con contenedores Linux y
PowerShell (incluido en Windows 10/11).

## 1. Requisitos

- Windows 10/11 de 64 bits.
- Docker Desktop instalado y abierto.
- En Docker Desktop: **Settings → General → Use the WSL 2 based engine**.
- Asegurarse de estar usando **contenedores Linux** (icono de Docker Desktop).
- PowerShell 5.1 o superior (viene con Windows).

No hace falta instalar Go para ejecutar la demo con Docker. Solo se necesita Go
si se quieren correr las pruebas unitarias en el host.

## 2. Abrir el proyecto

Descomprimir el ZIP y abrir **PowerShell** dentro de la carpeta del proyecto:

```powershell
cd C:\ruta\saga-choreography-go
```

Verificar que Docker responde:

```powershell
docker version
docker compose version
```

Si `docker` no se reconoce, abrir Docker Desktop y esperar a que termine de
iniciar.

## 3. Crear el archivo de configuración

```powershell
Copy-Item .env.example .env
```

## 4. Levantar todo

La forma más sencilla:

```powershell
.\scripts\saga.cmd up
```

Ese comando usa automáticamente:

- `docker-compose.yml`
- `docker-compose.windows.yml` (elimina el sufijo SELinux `,Z`, que no aplica
  en Windows)

Equivale a:

```powershell
docker compose -f docker-compose.yml -f docker-compose.windows.yml up -d --build
```

El primer arranque puede tardar varios minutos porque descarga imágenes y
compila los 4 servicios.

Comprobar el estado:

```powershell
.\scripts\saga.cmd ps
```

Deben aparecer 8 contenedores: RabbitMQ, PostgreSQL, los 4 servicios,
Prometheus y Grafana. Los servicios con healthcheck deben estar `healthy`.

## 5. Abrir la aplicación

| Recurso | URL |
|---|---|
| API de pedidos | http://localhost:8081/orders |
| RabbitMQ | http://localhost:15672 (`guest` / `guest`) |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 (`admin` / `admin`) |

Healthchecks:

```powershell
Invoke-RestMethod http://localhost:8081/health
Invoke-RestMethod http://localhost:8082/health
Invoke-RestMethod http://localhost:8083/health
Invoke-RestMethod http://localhost:8084/health
```

## 6. Ejecutar los escenarios

```powershell
.\scripts\saga.cmd demo-ok
.\scripts\saga.cmd demo-payment-fail
.\scripts\saga.cmd demo-inventory-fail
.\scripts\saga.cmd demo-shipping-fail
```

También se puede llamar directamente al script E2E:

```powershell
.\scripts\e2e.cmd ok
.\scripts\e2e.cmd payment-fail
.\scripts\e2e.cmd inventory-fail
.\scripts\e2e.cmd shipping-fail
```

O con parámetros de PowerShell:

```powershell
.\scripts\e2e.ps1 -Scenario shipping-fail -TimeoutSec 90
```

## 7. Logs

Seguir todos los logs:

```powershell
.\scripts\saga.cmd logs
```

Ver solo un servicio:

```powershell
docker compose -f docker-compose.yml -f docker-compose.windows.yml logs -f order-service
```

### Traza legible de una saga

Último pedido creado:

```powershell
.\scripts\trace.cmd
```

Un pedido concreto:

```powershell
.\scripts\trace.cmd <order_id>
```

También:

```powershell
.\scripts\saga.cmd trace
.\scripts\saga.cmd trace <order_id>
```

## 8. Pruebas (opcional, requiere Go)

Si se instala Go 1.23 o superior:

```powershell
.\scripts\saga.cmd test
.\scripts\saga.cmd cover-core
.\scripts\saga.cmd integration
```

`integration` requiere que el stack esté levantado.

## 9. Reiniciar la demo

Limpiar pedidos, pagos, envíos y restaurar stock sin borrar contenedores:

```powershell
.\scripts\saga.cmd reset
```

Detener todo y borrar volúmenes:

```powershell
.\scripts\saga.cmd down
```

## Problemas comunes

### `docker` no se reconoce

Abrir Docker Desktop y esperar a que el motor termine de iniciar. Luego probar:

```powershell
docker version
```

### Error de Execution Policy con PowerShell

Usar los wrappers `.cmd`, que ya incluyen `-ExecutionPolicy Bypass`:

```powershell
.\scripts\saga.cmd up
```

O ejecutar:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\saga.ps1 up
```

### Puerto ocupado

Ver qué proceso usa el puerto:

```powershell
netstat -ano | findstr :8081
```

Cerrar ese proceso o cambiar el puerto en `docker-compose.yml` y en la URL de
`scripts/e2e.ps1`.

### Docker Desktop está en contenedores Windows

Las imágenes del proyecto son Linux. Cambiar a **contenedores Linux** desde el
menú de Docker Desktop y volver a ejecutar.

### RabbitMQ no arranca

Ver los logs:

```powershell
docker compose -f docker-compose.yml -f docker-compose.windows.yml logs rabbitmq
```

El proyecto fuerza `user: "0:0"` y `entrypoint: ["rabbitmq-server"]` para evitar
problemas de permisos en volúmenes. Si se cambió esa configuración, restaurarla.

### La base no tiene las 4 bases de datos

El script `deploy/postgres/init.sql` solo se ejecuta cuando el volumen de
PostgreSQL está vacío. Para recrear todo:

```powershell
.\scripts\saga.cmd down
.\scripts\saga.cmd up
```
