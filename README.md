# Proyecto-Sistemas-Distribuidos

Hola estimados

## Levantar con Docker Compose

1. Copia el archivo de ejemplo a `.env`:

```bash
cp .env.example .env
```

2. Levanta los servicios:

```bash
docker compose up --build
```

## Arquitectura

- `user-service`: gestiona alta y consulta de usuarios.
- `catalog-service`: expone comercios, menus y productos.
- `orders-service`: crea y consulta pedidos.
- `payments-service`: procesa y consulta pagos.
- `api-gateway`: unico punto de entrada HTTP/REST; traduce a gRPC hacia los servicios internos.

## Red interna

Los servicios internos no se publican al host. Solo el `api-gateway` expone un puerto externo.

## Acoplamiento

Cada servicio usa su propia base de datos y su propio contrato gRPC. No se comparten tablas, ni structs internos, ni acceso directo entre bases.

## Justificacion de limites

- `user-service` existe para concentrar identidad y perfil de usuario.
- `catalog-service` existe para aislar el dominio de comercios y productos.
- `orders-service` existe para manejar el ciclo de vida de pedidos sin mezclar persistencia de catálogos o pagos.
- `payments-service` existe para encapsular el flujo de cobros y su estado.
- `api-gateway` existe para separar la entrada HTTP pública de los contratos gRPC internos.

## Probar API Gateway

1. Levanta el stack:

```bash
docker compose up --build
```

2. Verifica salud:

```bash
curl http://localhost:8080/healthz
```

3. Prueba un endpoint:

```bash
curl http://localhost:8080/catalog/comercios
```

4. Ejemplos adicionales:

```bash
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana","correo":"ana@mail.com","password":"123456","payment_token":"tok_123"}'
```

```bash
curl -X POST http://localhost:8080/payments/process \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"order-1","user_id":"user-1","amount":1000,"metodo_pago_token":"tok_123"}'
```

## Endpoints del Gateway

- `GET /healthz`
- `GET /`
- `POST /users`
- `GET /users/{id}`
- `GET /catalog/comercios`
- `GET /catalog/comercios/{id}/menu`
- `GET /catalog/products/{id}`
- `POST /orders`
- `GET /orders/{id}`
- `POST /orders/pickup/confirm`
- `POST /payments/process`
- `GET /payments/order/{order_id}`

## Curl por endpoint

```bash
curl http://localhost:8080/
```

```bash
curl http://localhost:8080/healthz
```

```bash
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana","correo":"ana@mail.com","password":"123456","payment_token":"tok_123"}'
```

```bash
curl http://localhost:8080/users/USER_ID
```

```bash
curl http://localhost:8080/catalog/comercios
```

```bash
curl "http://localhost:8080/catalog/comercios/COMERCIO_ID/menu"
```

```bash
curl http://localhost:8080/catalog/products/PRODUCT_ID
```

```bash
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"USER_ID","comercio_id":"COMERCIO_ID","items":[{"producto_id":"PRODUCT_ID","cantidad":2}]}'
```

```bash
curl http://localhost:8080/orders/ORDER_ID
```

```bash
curl -X POST http://localhost:8080/orders/pickup/confirm \
  -H 'Content-Type: application/json' \
  -d '{"qr_retiro":"QR_CODE"}'
```

```bash
curl -X POST http://localhost:8080/payments/process \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ORDER_ID","user_id":"USER_ID","amount":1000,"metodo_pago_token":"tok_123"}'
```

```bash
curl http://localhost:8080/payments/order/ORDER_ID
```
