FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /bin/catalog-service .

FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /bin/catalog-service /usr/local/bin/catalog-service

EXPOSE 50051

CMD ["/usr/local/bin/catalog-service"]
