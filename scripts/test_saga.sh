#!/bin/bash

# Script de prueba para el flujo SAGA en FoodRush
# Requiere: curl, jq (opcional para formateo)

# IDs de la semilla (init.sql)
COMERCIO_ID="c1111111-1111-1111-1111-111111111111"
PRODUCTO_ID="11111111-1111-1111-1111-111111111111"
USER_ID="00000000-0000-0000-0000-000000000001" # Ajustado a UUID

GATEWAY_URL=${GATEWAY_URL:-"http://localhost:8080"}
WAIT_TIME=10 # Tiempo para que los procesos asíncronos (Relay/Kafka) completen

echo "=== Iniciando Lote de Pruebas SAGA - FoodRush ==="

# 1. CASO FELIZ: Orden con Pago Aprobado
echo -e "\n--- Caso 1: Flujo Feliz (Pago Aprobado) ---"
echo "1.1 Creando orden para usuario..."
ORDER_RESP=$(curl -v -s -X POST "$GATEWAY_URL/orders" \
     -H "Content-Type: application/json" \
     -d "{
       \"user_id\": \"$USER_ID\",
       \"comercio_id\": \"$COMERCIO_ID\",
       \"items\": [
         {\"producto_id\": \"$PRODUCTO_ID\", \"cantidad\": 2}
       ]
     }" 2>&1)

ORDER_ID=$(echo $ORDER_RESP | grep -oP '"id":"\K[^"]+')
if [ -z "$ORDER_ID" ]; then
    echo "Error al crear la orden: $ORDER_RESP"
    exit 1
fi
echo "Orden creada: $ORDER_ID. Estado inicial: CREATED"

echo "Esperando $WAIT_TIME segundos para procesamiento SAGA..."
sleep $WAIT_TIME

echo "1.2 Verificando estado final de la orden..."
FINAL_ORDER=$(curl -s -X GET "$GATEWAY_URL/orders/$ORDER_ID")
FINAL_STATUS=$(echo $FINAL_ORDER | grep -oP '"status":"\K[^"]+')

echo "Resultado Caso 1:"
echo "ID: $ORDER_ID"
echo "Estado Final: $FINAL_STATUS"
if [ "$FINAL_STATUS" == "PAID" ]; then
    echo "✅ ÉXITO: La saga completó el flujo feliz."
else
    echo "❌ FALLO: Se esperaba PAID pero se obtuvo $FINAL_STATUS"
fi


# 2. CASO FALLO DE NEGOCIO: Orden con Pago Rechazado
echo -e "\n--- Caso 2: Fallo de Negocio (Pago Rechazado) ---"
echo "Nota: El sistema simula fallo si el total > 1000"
echo "2.1 Creando orden de alto valor..."
ORDER_RESP_FAIL=$(curl -s -X POST "$GATEWAY_URL/orders" \
     -H "Content-Type: application/json" \
     -d "{
       \"user_id\": \"$USER_ID\",
       \"comercio_id\": \"$COMERCIO_ID\",
       \"items\": [
         {\"producto_id\": \"$PRODUCTO_ID\", \"cantidad\": 1000}
       ]
     }")

ORDER_ID_FAIL=$(echo $ORDER_RESP_FAIL | grep -oP '"id":"\K[^"]+')
echo "Orden creada: $ORDER_ID_FAIL. Esperando compensación..."
sleep $WAIT_TIME

echo "2.2 Verificando estado de compensación..."
FINAL_ORDER_FAIL=$(curl -s -X GET "$GATEWAY_URL/orders/$ORDER_ID_FAIL")
FINAL_STATUS_FAIL=$(echo $FINAL_ORDER_FAIL | grep -oP '"status":"\K[^"]+')

echo "Resultado Caso 2:"
echo "ID: $ORDER_ID_FAIL"
echo "Estado Final: $FINAL_STATUS_FAIL"
if [ "$FINAL_STATUS_FAIL" == "PAYMENT_DECLINED" ]; then
    echo "✅ ÉXITO: La transacción compensatoria se ejecutó correctamente."
else
    echo "❌ FALLO: Se esperaba PAYMENT_DECLINED pero se obtuvo $FINAL_STATUS_FAIL"
fi


# 3. CASO RESILIENCIA: Servicio Caído (Manual)
echo -e "\n--- Caso 3: Resiliencia (Simulación de Outbox) ---"
echo "Para probar este caso manualmente:"
echo "1. Detén el contenedor de payment-service: docker-compose stop payment-service"
echo "2. Crea una orden nueva."
echo "3. Verifica que la orden sigue en CREATED (el evento está en la Outbox)."
echo "4. Inicia payment-service: docker-compose start payment-service"
echo "5. Verifica que la orden cambia a PAID/PAYMENT_DECLINED automáticamente."
echo "------------------------------------------------"

echo -e "\n=== Pruebas Finalizadas ==="
