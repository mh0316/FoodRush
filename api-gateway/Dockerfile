FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app

# Copiamos el gateway, todos los servicios que importa como replace locales
# y el módulo compartido de observabilidad.
COPY api-gateway ./api-gateway
COPY user-service ./user-service
COPY catalog-service ./catalog-service
COPY order-service ./order-service
COPY payment-service ./payment-service
COPY observability/go ./observability/go

WORKDIR /app/api-gateway
RUN go mod download
RUN go build -o api-gateway .

FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=builder /app/api-gateway/api-gateway .

EXPOSE 8080
CMD ["./api-gateway"]
