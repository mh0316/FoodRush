# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app/order-service

RUN apk add --no-cache git

COPY order-service/go.mod order-service/go.sum ./
RUN go mod download

COPY order-service ./
COPY catalog-service ../catalog-service
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o orders-service .

# Final stage
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/order-service/orders-service .

EXPOSE 50051

CMD ["./orders-service"]
