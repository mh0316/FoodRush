FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app

# Copiamos archivos de dependencias
COPY go.mod go.sum ./

# Copiamos el código
COPY . .

# Forzamos una versión moderna de gRPC que funcione con Go 1.23
RUN go get google.golang.org/grpc@v1.64.0
RUN go mod tidy

# Compilamos
RUN go build -o main .

# Fase de ejecución
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 50051
CMD ["./main"]
