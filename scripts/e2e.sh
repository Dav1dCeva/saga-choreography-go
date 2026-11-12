#!/usr/bin/env bash
# Verifica de punta a punta los cuatro escenarios de la saga.
set -euo pipefail

ORDER_URL="${ORDER_URL:-http://localhost:8081}"
SCENARIO="${1:-ok}"
TIMEOUT="${E2E_TIMEOUT:-60}"

command -v curl >/dev/null 2>&1 || { echo "Falta curl" >&2; exit 1; }
command -v python3 >/dev/null 2>&1 || { echo "Falta python3" >&2; exit 1; }

json_get() {
  python3 -c 'import json,sys; data=json.load(sys.stdin); print(data.get(sys.argv[1], ""))' "$1"
}

case "$SCENARIO" in
  ok)
    PAYLOAD='{"customer":"Ana Torres","product":"phone","quantity":1,"unitPrice":400,"address":"Av. Siempre Viva 742"}'
    EXPECTED="COMPLETED"
    ;;
  payment-fail)
    PAYLOAD='{"customer":"Ana Torres","product":"phone","quantity":1,"unitPrice":1500,"address":"Av. Siempre Viva 742"}'
    EXPECTED="CANCELLED"
    ;;
  inventory-fail)
    PAYLOAD='{"customer":"Ana Torres","product":"laptop","quantity":99,"unitPrice":1,"address":"Av. Siempre Viva 742"}'
    EXPECTED="CANCELLED"
    ;;
  shipping-fail)
    PAYLOAD='{"customer":"Ana Torres","product":"phone","quantity":1,"unitPrice":400,"address":"direccion invalid"}'
    EXPECTED="CANCELLED"
    ;;
  *)
    echo "Escenario desconocido: $SCENARIO" >&2
    echo "Use: ok | payment-fail | inventory-fail | shipping-fail" >&2
    exit 1
    ;;
esac

if ! RESPONSE="$(curl -fsS -X POST "$ORDER_URL/orders" -H 'Content-Type: application/json' -d "$PAYLOAD")"; then
  echo "No se pudo crear el pedido. ¿Está levantado order-service en $ORDER_URL?" >&2
  exit 1
fi

ORDER_ID="$(printf '%s' "$RESPONSE" | json_get id)"
if [[ -z "$ORDER_ID" ]]; then
  echo "Respuesta inesperada al crear el pedido: $RESPONSE" >&2
  exit 1
fi

echo "Escenario: $SCENARIO"
echo "Pedido creado: $ORDER_ID"
echo "Estado esperado: $EXPECTED"

LAST=""
DEADLINE=$((SECONDS + TIMEOUT))
while (( SECONDS < DEADLINE )); do
  if ORDER="$(curl -fsS "$ORDER_URL/orders/$ORDER_ID" 2>/dev/null)"; then
    STATUS="$(printf '%s' "$ORDER" | json_get status)"
    if [[ "$STATUS" != "$LAST" ]]; then
      echo "  estado: $STATUS"
      LAST="$STATUS"
    fi
    if [[ "$STATUS" == "$EXPECTED" ]]; then
      echo "OK: $ORDER_ID terminó en $STATUS"
      echo "$ORDER"
      exit 0
    fi
  fi
  sleep 2
done

echo "TIMEOUT: el pedido $ORDER_ID quedó en ${LAST:-desconocido} y se esperaba $EXPECTED" >&2
exit 1
