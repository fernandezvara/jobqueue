FROM golang:1.23.2-alpine AS builder

WORKDIR /app

# Instalar dependencias necesarias
RUN apk add --no-cache gcc musl-dev

# Copiar los archivos go.mod y go.sum
COPY go.mod go.sum ./

# Descargar dependencias
RUN go mod download

# Copiar el código fuente
COPY . .

# Compilar la aplicación
RUN CGO_ENABLED=0 GOOS=linux go build -a -o jobqueue ./cmd/jobqueue

# Imagen final
FROM alpine:latest

WORKDIR /app

# Copiar el binario compilado
COPY --from=builder /app/jobqueue .

# Exponer el puerto
EXPOSE 8080

# Comando para ejecutar la aplicación
CMD ["./jobqueue"]