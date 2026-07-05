# Documento Técnico

## Descripción Del Sistema

FoodRush es un sistema distribuido orientado a la gestión de pedidos para comercios gastronómicos. Su objetivo es exponer una interfaz pública simple para consumidores finales, mientras que la lógica de negocio se descompone en servicios autónomos de usuarios, catálogo, pedidos y pagos, coordinados internamente mediante gRPC y un API Gateway HTTP.

## Arquitectura

```mermaid
flowchart LR
    Client[Cliente HTTP]
    APIGW[API Gateway]

    UserSvc[User Service]
    CatalogSvc[Catalog Service]
    OrdersSvc[Orders Service]
    PaymentsSvc[Payments Service]

    UserDB[(User DB)]
    CatalogDB[(Catalog DB)]
    OrdersDB[(Orders DB)]
    PaymentsDB[(Payments DB)]

    UserDBNote[PostgreSQL]
    CatalogDBNote[PostgreSQL]
    OrdersDBNote[MongoDB]
    PaymentsDBNote[PostgreSQL]

    Client -->|REST| APIGW
    APIGW -->|gRPC| UserSvc
    APIGW -->|gRPC| CatalogSvc
    APIGW -->|gRPC| OrdersSvc
    APIGW -->|gRPC| PaymentsSvc

    UserSvc --> UserDB
    CatalogSvc --> CatalogDB
    OrdersSvc --> OrdersDB
    PaymentsSvc --> PaymentsDB

    UserDB --> UserDBNote
    CatalogDB --> CatalogDBNote
    OrdersDB --> OrdersDBNote
    PaymentsDB --> PaymentsDBNote
```

### Persistencia Por Servicio

```mermaid
flowchart TB
    UserSvc[User Service] --> UserPG[(PostgreSQL\nuser-database)]
    CatalogSvc[Catalog Service] --> CatalogPG[(PostgreSQL\ncatalog-database)]
    OrdersSvc[Orders Service] --> OrdersMG[(MongoDB\norders-database)]
    PaymentsSvc[Payments Service] --> PaymentsPG[(PostgreSQL\npayment-database)]
```

## Servicios Del Sistema

- `user-service`: administra el alta y la consulta del perfil de usuario.
- `catalog-service`: concentra el dominio de comercios, menús y productos.
- `orders-service`: gestiona la creación, consulta y confirmación de pedidos.
- `payments-service`: encapsula el procesamiento y la consulta del estado de pago.
- `api-gateway`: actúa como punto de entrada HTTP/REST y traduce solicitudes hacia los contratos gRPC internos.

## Red Interna

Los servicios internos no se exponen al host. La comunicación entre contenedores se realiza mediante nombres de servicio de Docker, lo que evita dependencias de IPs fijas y preserva el aislamiento de la red privada del stack.

## Acoplamiento

Cada servicio posee su propia base de datos y su propio contrato gRPC. No existen tablas compartidas, estructuras internas reutilizadas entre dominios ni acceso directo entre bases de datos. Esta decisión reduce el acoplamiento estructural y facilita la evolución independiente de cada frontera funcional.

## Justificacion De Limites

- `user-service` existe para aislar la gestión de identidad, autenticación funcional y atributos de perfil.
- `catalog-service` existe para delimitar el subdominio de oferta comercial, evitando mezclarlo con la lógica transaccional.
- `orders-service` existe para encapsular el ciclo de vida del pedido, su estado y su persistencia en una colección transaccional distinta.
- `payments-service` existe para modelar el flujo de cobro como una responsabilidad separada del pedido, reduciendo dependencia temporal y conceptual.
- `api-gateway` existe para desacoplar la interfaz pública HTTP de la comunicación interna gRPC y concentrar la traducción de protocolos en un único punto.

## Casos De Uso Y Flujos

### 1. Registrar Usuario

El cliente invoca `POST /users` con los atributos `nombre`, `correo`, `password` y `payment_token`. El gateway transforma la solicitud HTTP en la llamada gRPC `CreateUser` sobre `user-service`. Dicho servicio valida la integridad semántica de los datos y persiste el usuario en PostgreSQL. El resultado observable para el cliente es la creación de una entidad `User` con identificador persistido y estado `created`.

Flujo técnico:
- Cliente -> API Gateway por HTTP/REST
- API Gateway -> User Service por gRPC
- User Service -> PostgreSQL del dominio de usuarios
- Respuesta JSON al cliente con el recurso creado

### 2. Consultar Catálogo

El cliente consulta `GET /catalog/comercios`, `GET /catalog/comercios/{id}/menu` o `GET /catalog/products/{id}`. El gateway reenvía la operación al `catalog-service`, que resuelve la lectura desde su base de datos PostgreSQL. El resultado esperado es la lista de comercios activos, el menú asociado a un comercio o el detalle técnico de un producto.

Flujo técnico:
- Cliente -> API Gateway
- API Gateway -> Catalog Service por gRPC
- Catalog Service -> PostgreSQL del dominio de catálogo
- Respuesta JSON al cliente

### 3. Crear Pedido

El cliente envía `POST /orders` con `user_id`, `comercio_id` e `items`. El gateway delega en `orders-service` la operación `CreateOrder`. Antes de persistir, el servicio consulta a `catalog-service` por cada `producto_id` para obtener el precio vigente y calcular el importe total con datos reales del catálogo. Luego normaliza el pedido y lo persiste en MongoDB. El resultado esperado es un pedido creado con identificador propio, monto total y estado inicial de negocio.

Flujo técnico:
- Cliente -> API Gateway
- API Gateway -> Orders Service por gRPC
- Orders Service -> Catalog Service por gRPC para resolver precios
- Orders Service -> MongoDB del dominio de pedidos
- Respuesta JSON al cliente

### 4. Procesar Pago

El cliente envía `POST /payments/process` con `order_id`, `user_id`, `amount` y `metodo_pago_token`. El gateway invoca `ProcessPayment` sobre `payments-service`, que encapsula la ejecución del flujo de cobro y retorna un estado de aprobación o rechazo. El resultado esperado es un registro de pago con estado explícito.

Flujo técnico:
- Cliente -> API Gateway
- API Gateway -> Payments Service por gRPC
- Payments Service ejecuta su lógica de cobro y responde
- Respuesta JSON al cliente

## Implementación de SAGA y Patrón Outbox

### Descripción del Flujo SAGA (Coreografía)
Se ha implementado una Saga coreografiada para el flujo crítico de Creación y Pago de Pedido. El sistema utiliza Kafka como backbone de eventos para coordinar los servicios de forma asíncrona y desacoplada.

1.  **Creación (Order Service):** El servicio recibe una solicitud gRPC, valida el catálogo, y persiste la orden en estado `CREATED`. En la misma operación atómica (dentro del documento MongoDB), guarda un evento `foodrush.orders.created` en una lista "outbox".
2.  **Relay & Publicación:** Un proceso Relay independiente en `order-service` escanea periódicamente la base de datos, publica los eventos pendientes en Kafka (incluyendo el `correlation_id` en los Headers) y los marca como procesados.
3.  **Procesamiento de Pago (Payment Service):** El consumidor de Kafka en `payment-service` detecta el evento. Procesa el pago y, en una transacción atómica de PostgreSQL, persiste el registro del pago y guarda un evento `foodrush.payments.processed` en su propia tabla `outbox`.
4.  **Relay de Pago:** El Relay de `payment-service` publica el resultado en Kafka.
5.  **Finalización (Order Service):** El consumidor de `order-service` recibe el resultado del pago. Si es `APPROVED`, actualiza la orden a `PAID`. Si es `DECLINED` (transacción compensatoria), actualiza la orden a `PAYMENT_DECLINED`.

### Decisión Técnica Clave: Patrón Outbox con Relay Dedicado
*   **Decisión:** Se implementó el Patrón Outbox en lugar de publicar directamente a Kafka desde la lógica del servicio. En MongoDB (`order-service`) se integró la Outbox dentro del documento de la orden, mientras que en PostgreSQL (`payment-service`) se usó una tabla adicional dentro de la misma transacción.
*   **Alternativa descartada:** Publicación directa ("Dual Write").
*   **Razonamiento:** La publicación directa no garantiza atomicidad; el servicio podría persistir en la DB y fallar antes de publicar en Kafka, dejando el sistema en un estado inconsistente permanentemente. El Patrón Outbox garantiza entrega "at least once" al persistir el evento junto con el cambio de estado de negocio.

### Resiliencia y Comportamiento ante Fallos
*   **Kafka Down:** Si el broker de Kafka es inalcanzable, los procesos Relay reintentarán la publicación indefinidamente sin afectar la disponibilidad de la base de datos o la respuesta inmediata al usuario.
*   **Servicio Caído:** Si un servicio consumidor está caído, Kafka retiene los mensajes hasta que el servicio se recupere, asegurando que el flujo SAGA eventualmente continúe.
*   **Error de Negocio (Pago Rechazado):** Se maneja como una transacción compensatoria, moviendo la orden al estado `PAYMENT_DECLINED`, liberando el flujo de forma consistente.

### Limitaciones y Trade-offs
*   **Limitación:** Latencia de "Casi-Real-Time".
*   **Trade-off:** La introducción del Relay añade un pequeño retraso (polling interval) entre la persistencia y la visibilidad del evento en Kafka. Se aceptó este compromiso a cambio de la garantía de consistencia eventual y la eliminación del riesgo de pérdida de eventos.
*   **Headers de Kafka:** El `correlation_id` viaja exclusivamente en los Kafka Headers, lo que optimiza el ruteo y observabilidad en infraestructura, pero requiere que todos los clientes Kafka sean compatibles con esta funcionalidad.

## Decisiones Tecnicas Y Trade-offs

### Base De Datos Por Servicio

Se adoptó una base de datos por servicio para evitar acoplamiento de persistencia y permitir evolución independiente de cada subdominio. El beneficio principal es el aislamiento de responsabilidades, el ownership claro de los datos y la reducción del riesgo de regresiones cruzadas. El costo es operativo: se incrementa el número de contenedores, variables de entorno y puntos de observación.

### API Gateway Como Entrada Unica

Se eligió un API Gateway HTTP/REST para ofrecer una interfaz pública uniforme y mantener gRPC como contrato interno entre servicios. La ventaja es una superficie de consumo más sencilla y un punto único para políticas transversales. La desventaja es una capa adicional en la ruta de petición y un componente más a mantener.

### gRPC Interno Con Protobuf

Se adoptó gRPC con Protobuf para la comunicación interna debido a su tipado fuerte, bajo costo de serialización y generación de clientes/servidores a partir de contrato. Esto mejora la consistencia del intercambio entre servicios. A cambio, se pierde legibilidad directa frente a JSON y se introduce una etapa adicional de generación de código.

### Servicios Separados Por Dominio

La separación en usuarios, catálogo, pedidos y pagos responde a límites de negocio observables y no a una división accidental del código. Esta decisión incrementa la cohesión interna de cada servicio y reduce el riesgo de mezclar responsabilidades. El costo es una mayor coordinación entre componentes y un mayor esfuerzo inicial de integración.

En el caso de pedidos, existe una dependencia de lectura controlada hacia catálogo para resolver precios actuales. `orders-service` consulta a `catalog-service` durante `CreateOrder` para obtener el precio real de cada producto y calcular el total con datos vigentes. Esta decisión mejora la coherencia del negocio, pero introduce una dependencia temporal entre ambos servicios durante la creación de una orden.

Cuando catálogo no responde, `orders-service` reintenta la lectura unas pocas veces y devuelve un error controlado al caller en lugar de colgarse.

### Persistencia Heterogenea

Se emplea PostgreSQL para usuarios, catálogo y pagos, y MongoDB para pedidos. La elección responde a patrones de acceso distintos: consultas relacionales y consistencia estructurada en unos dominios, y documentos flexibles en el caso de pedidos. El beneficio es una representación más natural de cada entidad; el costo es la heterogeneidad tecnológica y una mayor complejidad operativa.

### Persistencia Por Dominio

- Usuarios: se persisten en PostgreSQL del `user-service`.
- Catálogo: comercios y productos se persisten en PostgreSQL del `catalog-service`.
- Pedidos: se persisten en MongoDB del `orders-service`.
- Pagos: el `payments-service` encapsula el flujo de cobro y su estado dentro de su propio contrato.
