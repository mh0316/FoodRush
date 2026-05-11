FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app

COPY api-gateway ./api-gateway
COPY user-service ./user-service
COPY catalog-service ./catalog-service
COPY order-service ./order-service
COPY payment-service ./payment-service

WORKDIR /app/api-gateway
RUN go mod download
RUN go build -o api-gateway .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/api-gateway/api-gateway .
EXPOSE 8080
CMD ["./api-gateway"]
