FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app

# Copiamos el módulo compartido de observabilidad y el servicio.
COPY observability/go ./observability/go
COPY catalog-service ./catalog-service

WORKDIR /app/catalog-service
RUN go mod download

RUN go build -o main .

FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /root/
COPY --from=builder /app/catalog-service/main .

EXPOSE 50051 9000
CMD ["./main"]
