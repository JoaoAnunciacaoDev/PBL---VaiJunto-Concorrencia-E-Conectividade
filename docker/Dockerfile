
# --- Estágio de compilação ---
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /app/bin \
    && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api-server ./cmd/api-server \
    && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api-client ./cmd/api-client \
    && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/seed ./cmd/seed \
    && CGO_ENABLED=0 GOOS=linux go build -o /app/bin/simulator ./cmd/simulator

# --- Estágio final ---
FROM alpine:latest

WORKDIR /app

# Cria usuário sem privilégios e diretórios persistentes
RUN addgroup -S vaijunto \
    && adduser -S -G vaijunto vaijunto \
    && mkdir -p /app/storage/data /app/storage/logs /app/storage/wal \
    && chown -R vaijunto:vaijunto /app/storage

COPY --from=builder --chown=vaijunto:vaijunto /app/bin/ /app/bin/

# Todos os processos executam sem privilégios de root
USER vaijunto

EXPOSE 8080 8081

VOLUME ["/app/storage"]

CMD ["/app/bin/api-server", "-data-dir", "/app/storage/data"]
