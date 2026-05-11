FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /bin/orders-service .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /bin/orders-service /usr/local/bin/orders-service

EXPOSE 50051

CMD ["/usr/local/bin/orders-service"]
