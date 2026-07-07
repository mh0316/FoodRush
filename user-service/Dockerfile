FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copiamos el módulo compartido de observabilidad y el servicio.
COPY observability/go ./observability/go
COPY user-service ./user-service

WORKDIR /app/user-service
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/user-service .

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /bin/user-service /usr/local/bin/user-service

EXPOSE 50051 9000

CMD ["/usr/local/bin/user-service"]
