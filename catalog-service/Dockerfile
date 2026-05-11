FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app

# Copiamos archivos de dependencias
COPY go.mod go.sum ./

# Copiamos el código
COPY . .

# Compilamos
RUN go build -o main .

# Fase de ejecución
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 50052
CMD ["./main"]
