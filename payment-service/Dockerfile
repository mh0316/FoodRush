FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app

# Copiamos el módulo compartido de observabilidad y el servicio.
COPY observability/go ./observability/go
COPY payment-service ./payment-service

WORKDIR /app/payment-service
RUN go mod download

RUN go build -o payments-service .

FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=builder /app/payment-service/payments-service .

EXPOSE 50051 9000
CMD ["./payments-service"]
