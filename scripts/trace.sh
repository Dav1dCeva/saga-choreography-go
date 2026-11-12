#!/usr/bin/env bash
# Muestra la traza legible de una saga: una línea por evento, ordenada por tiempo.
# Uso: ./scripts/trace.sh <order_id>
#      ./scripts/trace.sh            (toma el último pedido creado)
# Variable opcional: COMPOSE="podman-compose" (por defecto autodetecta).
set -euo pipefail

if [ -z "${COMPOSE:-}" ]; then
  if command -v podman-compose >/dev/null 2>&1; then COMPOSE="podman-compose"
  else COMPOSE="docker compose"; fi
fi

SERVICES="order-service payment-service inventory-service shipping-service"
LOGS="$($COMPOSE logs --no-color $SERVICES 2>&1)"

ORDER_ID="${1:-}"
if [ -z "$ORDER_ID" ]; then
  ORDER_ID="$(printf '%s\n' "$LOGS" | grep '"type":"order.created"' | grep '"msg":"evento publicado"' | tail -n1 \
    | grep -o '"order_id":"[^"]*"' | head -n1 | cut -d'"' -f4)"
fi
[ -n "$ORDER_ID" ] || { echo "No se encontró ningún pedido en los logs." >&2; exit 1; }

PY=$(cat <<'PYEOF'
import sys, json, re

rows = []
for line in sys.stdin:
    m = re.search(r"\{.*\}", line)
    if not m:
        continue
    try:
        rows.append(json.loads(m.group(0)))
    except ValueError:
        continue

if not rows:
    sys.exit("Sin eventos para ese pedido.")
rows.sort(key=lambda d: d.get("time", ""))

print("SAGA " + rows[0].get("saga_id", ""))
print("%-13s%-11s%-4s%s" % ("HORA", "SERVICIO", "", "DETALLE"))
print("-" * 78)

for d in rows:
    hora = str(d.get("time", ""))[11:23]
    srv = str(d.get("service", "?")).replace("-service", "")
    msg = d.get("msg", "")
    if msg == "evento recibido":
        icono, txt = "<-", "recibe    " + d.get("type", "")
    elif msg == "evento publicado":
        icono, txt = "->", "publica   " + d.get("type", "")
    elif msg == "acción ejecutada":
        icono = "**"
        txt = "ACCIÓN    " + d.get("action", "")
        if d.get("estado"):
            txt += "  => estado " + d["estado"]
        elif d.get("detalle") or d.get("product"):
            txt += "  (" + (d.get("detalle") or d.get("product")) + ")"
        if d.get("motivo"):
            txt += "  [motivo: " + d["motivo"] + "]"
    elif msg == "saga completada":
        icono, txt = "OK", "SAGA COMPLETADA  tracking " + d.get("tracking", "")
    elif d.get("estado") == "CANCELLED":
        icono, txt = "XX", "SAGA CANCELADA"
    else:
        icono, txt = "  ", msg
    print("%-13s%-11s%-4s%s" % (hora, srv, icono, txt))
PYEOF
)

printf '%s\n' "$LOGS" | grep "$ORDER_ID" | python3 -c "$PY"
