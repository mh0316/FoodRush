# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Install git for downloading dependencies
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o orders-service .

# Final stage
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/orders-service .

EXPOSE 50051

CMD ["./orders-service"]
