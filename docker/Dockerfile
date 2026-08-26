# Multi-stage build para gerar contêineres Docker ultraleves e seguros
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copia arquivos de dependência e baixa os pacotes
COPY go.mod go.sum ./
RUN go mod download

# Copia todo o código fonte
COPY . .

# Compila todos os binários da aplicação
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api-server ./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/driver-client ./cmd/driver-client
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/passenger-client ./cmd/passenger-client
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/stress-tester ./cmd/stress-tester

# --- Estágio Final de Execução ---
FROM alpine:latest

WORKDIR /app

# Copia os binários compilados
COPY --from=builder /app/bin/ /app/bin/

# Copia a malha rodoviária da Bahia
COPY --from=builder /app/internal/data/ /app/internal/data/

# Expõe a porta TCP do servidor central
EXPOSE 8080

# Por padrão, roda o servidor central
CMD ["/app/bin/api-server"]