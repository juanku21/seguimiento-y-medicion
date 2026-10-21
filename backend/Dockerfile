# Imagen de la API del backend en dos etapas: la primera genera Swagger y compila un binario
# estático; la segunda solo lo ejecuta. Las versiones de las imágenes están fijadas (Principio IV).
# Construcción, desde backend/:
#   docker build -t smye-backend .
# Ejecución, con las variables de backend/.env.example cargadas en el entorno:
#   docker run --env-file .env -p 8080:8080 smye-backend

# Etapa de compilación: Go 1.27.3, la misma versión que declara go.mod.
FROM golang:1.27.3-alpine3.24 AS build
WORKDIR /src

# Se instala la CLI de swag en la misma versión que la librería de go.mod, para que la
# documentación generada coincida con la que produce el desarrollador.
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.6

# Se descargan las dependencias antes de copiar el código para aprovechar la caché de capas.
COPY go.mod go.sum ./
RUN go mod download

# Se copia el código y se genera backend/docs/, que no se versiona (research.md sección 10).
COPY . .
RUN swag init -g cmd/app/main.go -o docs

# Binario estático sin cgo, para que corra en una imagen mínima sin toolchain.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/app

# Etapa de ejecución: Alpine con versión fija y un usuario sin privilegios.
FROM alpine:3.24.2
RUN addgroup -S app && adduser -S -G app app
WORKDIR /app
COPY --from=build /out/app /app/app
USER app

# La configuración llega por variables de entorno (Principio VI); SERVER_PORT debe coincidir con el
# puerto publicado.
ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/app/app"]
