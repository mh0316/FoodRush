# FoodRush

##### Proyecto para asignatura: Sistemas Distribuidos y Escalables

## Levantar con Docker Compose

1. Copia el archivo de ejemplo a `.env`:

```bash
cp .env.example .env
```

2. Levanta los servicios:

```bash
docker compose up --build
```
ó dependiendo de tu versión de Docker Compose:

```bash
docker-compose up --build
```

## Variables de Entorno

Usa `.env.example` como referencia para completar `.env`.

## Probar API Gateway

Las rutas públicas no cambiaron; solo se mejoró la implementación interna de los servicios.

### Salud

```bash
curl http://localhost:8080/healthz
```

### Root

```bash
curl http://localhost:8080/
```

### Usuarios

```bash
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana","correo":"ana@mail.com","password":"123456","payment_token":"tok_123"}'
```

```bash
curl http://localhost:8080/users/USER_ID
```

### Catalogo

```bash
curl http://localhost:8080/catalog/comercios
```

```bash
curl "http://localhost:8080/catalog/comercios/COMERCIO_ID/menu"
```

```bash
curl http://localhost:8080/catalog/products/PRODUCT_ID
```

### Pedidos

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

### Pagos

```bash
curl -X POST http://localhost:8080/payments/process \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ORDER_ID","user_id":"USER_ID","amount":1000,"metodo_pago_token":"tok_123"}'
```

```bash
curl http://localhost:8080/payments/order/ORDER_ID
```

## Secuencia de ejemplo

Usa esta secuencia si quieres probar el sistema de punta a punta con valores de ejemplo:

1. Crear usuario:

```bash
curl -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"nombre":"Ana Perez","correo":"ana.perez@mail.com","password":"123456","payment_token":"tok_ana_123"}'
```

2. Consultar catálogo:

```bash
curl http://localhost:8080/catalog/comercios
```

3. Consultar menú del comercio demo:

```bash
curl http://localhost:8080/catalog/comercios/c1111111-1111-1111-1111-111111111111/menu
```

4. Consultar detalle del producto demo principal:

```bash
curl http://localhost:8080/catalog/products/11111111-1111-1111-1111-111111111111
```

5. Crear pedido usando el `user_id` devuelto por el paso 1:

```bash
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"USER_ID_DEVUELTO","comercio_id":"c1111111-1111-1111-1111-111111111111","items":[{"producto_id":"11111111-1111-1111-1111-111111111111","cantidad":2}]}'
```

6. Obtener detalle del pedido usando el `id` devuelto por el paso 5:

```bash
curl http://localhost:8080/orders/ORDER_ID_DEVUELTO
```

7. Confirmar retiro del pedido:

```bash
curl -X POST http://localhost:8080/orders/pickup/confirm \
  -H 'Content-Type: application/json' \
  -d '{"qr_retiro":"QR_REAL"}'
```

8. Procesar pago usando el `order_id` devuelto por el paso 5:

```bash
curl -X POST http://localhost:8080/payments/process \
  -H 'Content-Type: application/json' \
  -d '{"order_id":"ORDER_ID_DEVUELTO","user_id":"USER_ID_DEVUELTO","amount":20,"metodo_pago_token":"tok_ana_123"}'
```

9. Consultar pago por pedido:

```bash
curl http://localhost:8080/payments/order/ORDER_ID_DEVUELTO
```

## Documento Técnico

Ver en sección de `docs` el archivo `documento-tecnico.md`.

[Link al Documento tecnico](https://github.com/mh0316/FoodRush/blob/develop/docs/documento-tecnico.md)
