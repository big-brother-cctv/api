# Build stage
FROM golang:alpine AS build

WORKDIR /app

ENV CGO_ENABLED=0
ENV GOTOOLCHAIN=auto

# Instalar dependencias del sistema
RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compilar binario estático optimizado
RUN go build -ldflags="-s -w" -o /app/server ./cmd/server

# Runtime stage
FROM alpine:3.19 AS runtime

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=build /app/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]