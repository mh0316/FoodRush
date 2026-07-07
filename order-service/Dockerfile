FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

# Copiamos el módulo compartido de observabilidad y el servicio.
COPY observability/go ./observability/go
COPY order-service ./order-service

WORKDIR /app/order-service
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/orders-service .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /bin/orders-service /usr/local/bin/orders-service

EXPOSE 50051 9000

CMD ["/usr/local/bin/orders-service"]
