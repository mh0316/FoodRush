# Imagen oficial de Go
FROM golang:1.24

# Directorio de trabajo dentro del contenedor
WORKDIR /app

# Copiar archivos de dependencias primero
COPY go.mod go.sum ./

# Descargar dependencias
RUN go mod download

# Copiar el resto del proyecto
COPY . .

# Compilar aplicación
RUN go build -o payments-service .

# Exponer puerto gRPC
EXPOSE 50051

# Ejecutar aplicación
CMD ["./payments-service"]