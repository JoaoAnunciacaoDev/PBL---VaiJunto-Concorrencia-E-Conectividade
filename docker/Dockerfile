# Multi-stage build para gerar uma imagem única com todos os executáveis.
FROM golang:1.27-alpine AS builder

WORKDIR /app

# Copia arquivos de dependência e baixa os pacotes
COPY go.mod go.sum ./
RUN go mod download

# Copia todo o código fonte
COPY . .

# Compila somente os comandos existentes no projeto.
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api-server ./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api-client ./cmd/api-client
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/seed ./cmd/seed

# --- Estágio Final de Execução ---
FROM alpine:latest

WORKDIR /app

# O servidor grava dados e logs em /app/storage. O usuário sem privilégios é
# dono desse diretório, inclusive quando nenhum volume é montado.
RUN addgroup -S vaijunto && adduser -S -G vaijunto vaijunto \
    && mkdir -p /app/storage \
    && chown -R vaijunto:vaijunto /app/storage

COPY --from=builder --chown=vaijunto:vaijunto /app/bin/ /app/bin/

USER vaijunto

# Expõe a porta TCP do servidor central
EXPOSE 8080
VOLUME ["/app/storage"]

# Por padrão, roda o servidor central
CMD ["/app/bin/api-server", "-data-dir", "/app/storage/data"]
