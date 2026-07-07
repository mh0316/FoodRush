# Entrega 2: SAGA y Transaccional Outbox

## 1. Descripción del Bloque
Este bloque implementa la coordinación distribuida de la creación de pedidos y el procesamiento de pagos en FoodRush, garantizando la consistencia eventual entre los servicios de `order-service` y `payment-service`.

### Flujo Completo
1.  **Creación del Pedido (`order-service`):** Cuando un usuario crea una orden, el servicio valida precios contra el `catalog-service`. Si es exitoso, persiste la orden en MongoDB con estado `CREATED` e inserta un evento de "Orden Creada" en la lista `outbox` del mismo documento de forma atómica.
2.  **Publicación (`Relay`):** Un proceso independiente lee las órdenes con eventos no procesados y los publica en Kafka en el topic `foodrush.orders.created`.
3.  **Procesamiento de Pago (`payment-service`):** El consumidor de pagos recibe el evento. Si el monto es menor a 1000, el pago se aprueba; si es mayor o igual, se rechaza. En una transacción de PostgreSQL, se guarda el registro del pago y el evento resultado (`processed` o `failed`) en la tabla `outbox`.
4.  **Confirmación de Pago:** El `Relay` de pagos publica el resultado en Kafka.
5.  **Actualización de Orden:** El `order-service` consume el resultado y actualiza el estado de la orden a `PAID` o `PAYMENT_DECLINED`.

## 2. Decisiones Técnicas

### Decisión Clave: Patrón Transaccional Outbox
*   **Decisión:** Se implementó el patrón Outbox para la publicación de eventos en Kafka.
*   **Alternativa descartada:** Dual Write (publicar directamente en Kafka después de guardar en la DB).
*   **Razonamiento:** En un sistema distribuido, no es posible garantizar que una escritura en la DB y una publicación en Kafka ocurran de forma atómica sin usar transacciones distribuidas (muy costosas). Con Dual Write, si el servicio cae después de escribir en la DB pero antes de publicar, el pedido quedaría "colgado" para siempre. El patrón Outbox asegura que el evento se persista junto con los datos de negocio, y el Relay garantiza su entrega "al menos una vez" (at-least-once).

### Shard Key y Distribución de Datos
*   **Decisión:** Se utiliza el `order_id` (UUID) como clave de distribución lógica.
*   **Razonamiento:** El uso de un UUID garantiza una distribución uniforme de la carga en clústeres de MongoDB y PostgreSQL, evitando "hot partitions" que ocurrirían con claves secuenciales o basadas en fechas. En MongoDB, el evento Outbox vive dentro del documento de la orden, lo que permite atomicidad sin transacciones multi-documento.

### Comportamiento ante Fallos (Circuit Breaker y Retries)
*   **Decisión:** El `order-service` implementa una política de reintentos con timeout para la comunicación gRPC con el `catalog-service`.
*   **Razonamiento:** Para proteger el sistema de fallos en cascada, se configuró un umbral de 3 reintentos con un timeout de 2 segundos por intento. Si el `catalog-service` no responde después de estos intentos, el sistema aplica un "fail-fast", informando al usuario que el servicio no está disponible momentáneamente. Esto actúa como un Circuit Breaker manual que evita saturar el servicio de catálogo si este se encuentra bajo estrés.

## 3. Comportamiento ante Fallos

| Escenario | Comportamiento del Sistema |
| :--- | :--- |
| **Kafka Caído** | Los procesos Relay mantienen los eventos en la base de datos (MongoDB/Postgres). El sistema sigue aceptando pedidos. Una vez Kafka se recupera, los Relays retoman la publicación automáticamente desde el último punto. |
| **Servicio de Pagos Caído** | Los eventos `order.created` se acumulan en Kafka. Cuando el servicio se reinicia, procesa todos los mensajes pendientes (Backpressure management). La orden permanece en `CREATED` hasta que el servicio procese el mensaje. |
| **Error en Base de Datos** | La transacción falla y el evento no se guarda. El usuario recibe un error 500 y puede reintentar. No hay riesgo de eventos "huérfanos" porque el evento solo existe si la transacción de negocio fue exitosa. |
| **Reintento de Red en Relay** | El Relay utiliza un mecanismo de polling. Si falla la publicación, no marca el evento como procesado, asegurando que se intente nuevamente en el siguiente ciclo. |
| **Pago Rechazado (Lógica de Negocio)** | Se trata como un fallo esperado en la SAGA. El sistema emite un evento de fallo que el `order-service` usa para ejecutar la compensación, moviendo el estado a `PAYMENT_DECLINED`. |

# 4. Trade-offs y Limitaciones

En el diseño de este bloque se aceptaron conscientemente los siguientes trade-offs para balancear simplicidad, consistencia y rendimiento:

### 4.1. UUID vs Claves Secuenciales (Shard Key)
*   **Trade-off:** El uso de UUID v4 como `order_id` y shard key garantiza una distribución de escritura perfecta, pero degrada el rendimiento de consultas basadas en tiempo o rangos (ej. "listar órdenes más recientes").
*   **Justificación:** En un sistema de delivery, el patrón de acceso principal es por ID de orden (seguimiento) y la carga de escritura es crítica durante horas pico. Se aceptó este compromiso delegando las consultas analíticas de rango a réplicas de lectura o índices secundarios.

### 4.2. Entrega At-Least-Once vs Latencia
*   **Trade-off:** El patrón Transaccional Outbox introduce un pequeño delay (latencia del Relay) y la posibilidad de mensajes duplicados en Kafka si el Relay falla antes de confirmar el procesamiento en la DB.
*   **Justificación:** Se priorizó la **consistencia atómica** (que una orden no se quede sin pago) sobre la latencia de milisegundos. La duplicidad se mitiga mediante lógica de idempotencia en el `payment-service` y `order-service`.

### 4.3. Timeout Estático en Circuit Breaker
*   **Trade-off:** El timeout de 2 segundos y el límite de 3 reintentos para gRPC son valores estáticos que podrían ser demasiado conservadores o agresivos según la carga.
*   **Justificación:** Se optó por una configuración simple para evitar la complejidad de implementar un Circuit Breaker adaptativo (como Hystrix o Sentinel) en esta etapa, asegurando al menos una protección básica contra el "bloqueo de hilos" en el servicio de órdenes.
    