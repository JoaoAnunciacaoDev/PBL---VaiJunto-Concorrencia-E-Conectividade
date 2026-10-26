# VaiJunto — Consórcio Distribuído de Caronas Compartilhadas

Aplicação de caronas compartilhadas distribuída construída em Go para a disciplina de **Concorrência e Conectividade (PBL 2)**. 

O sistema implementa um **consórcio descentralizado** formado por múltiplas empresas independentes (`empresa-a`, `empresa-b`, `empresa-c`), onde passageiros podem buscar e reservar itinerários intermunicipais compostos por trechos operados por diferentes empresas parceiras, **sem qualquer banco de dados compartilhado, sem coordenador fixo central e sem sistemas de terceiros (sem Redis, ZooKeeper, etcd ou Consul)**.

A consistência e a atomicidade das reservas entre os múltiplos servidores são garantidas através de uma implementação própria de **Two-Phase Commit (2PC)**, **Relógios Lógicos de Lamport**, **Locks temporizados por TTL** e **Write-Ahead Log (WAL)** com sincronização síncrona em disco (`fsync`).

---

## 🏛️ Arquitetura Distribuída

Cada empresa do consórcio executa uma instância completa e isolada do servidor, possuindo:
1. **Porta TCP (ex: :8080):** Atendimento a clientes de terminal (passageiros e motoristas) via mensagens JSON delimitadas por quebra de linha.
2. **Porta HTTP REST (ex: :8081):** Comunicação inter-servidores via API REST com framework Gin Gonic (`/internal/v1/...`).
3. **Repositório Próprio com Mutexes Granulares:** Armazenamento isolado em disco (`state.json`), sem compartilhamento de dados entre empresas.
4. **Write-Ahead Log (WAL):** Log transacional persistente em disco (JSON Lines) para garantir durabilidade e recuperação após quedas.
5. **Relógio Lógico de Lamport:** Ordenação causal de eventos sem dependência de relógios físicos.
6. **LockStore com TTL:** Exclusão mútua distribuída com liberação automática de assentos após 10 segundos em caso de falha do coordenador.

```
       [Passageiro] ──TCP (:8080)──► [Servidor Empresa A]
                                            ▲  │
                       POST /prepare, commit│  │REST (:8081)
                                            │  ▼
                                     [Servidor Empresa B]
                                            ▲  │
                       POST /prepare, commit│  │REST (:8081)
                                            │  ▼
                                     [Servidor Empresa C]
```

---

## 📁 Estrutura do Projeto

```text
.
├── cmd/
│   ├── api-client/          # Cliente interativo de terminal TCP
│   ├── api-server/          # Servidor da empresa (TCP + REST Gin)
│   ├── seed/                # Povoamento de dados de teste/demonstração
│   └── simulator/           # Simulador de estresse com passageiros simultâneos
├── docker/
│   ├── Dockerfile           # Multi-stage build com binários estáticos
│   └── docker-compose.yaml  # Orquestração do consórcio com 3 empresas
├── internal/
│   ├── client/              # Comunicação TCP e interface de terminal do cliente
│   ├── consortium/          # O coração distribuído: Lamport, WAL, LockStore, 2PC
│   │   ├── coordinator.go   # Orquestrador do Two-Phase Commit
│   │   ├── lamport_clock.go # Relógio lógico de Lamport thread-safe (CAS)
│   │   ├── peer_registry.go # Registro dinâmico de empresas parceiras
│   │   ├── segment_lock.go  # Locks de pré-reserva com TTL e prioridade determinística
│   │   └── wal.go           # Write-Ahead Log com fsync síncrono
│   ├── enum/                # Cidades, perfis e status de reservas
│   ├── httpapi/             # API REST inter-servidores (Gin Gonic)
│   ├── models/              # Entidades de domínio (Ride, Stage, ReservedSegment)
│   ├── protocol/            # Protocolo TCP, envelopes, DTOs e framing
│   ├── server/              # Engine do servidor, repositório e mutexes granulares
│   └── utils/               # Utilitários de terminal e arquivos
├── tests/
│   └── integration/         # Testes automatizados sob concorrência e injeção de falhas
│       ├── double_booking_test.go   # Validação de zero double-booking (21 concorrentes)
│       ├── infra.go                 # Infraestrutura de cluster em-processo
│       ├── partial_itinerary_test.go# Prova de atomicidade em itinerários multitrecho
│       └── recovery_test.go         # Recuperação de assentos bloqueados pós-crash
├── go.mod                   # Definição do módulo Go
├── go.sum                   # Checksums das dependências
├── PROTOCOL.md              # Documentação das mensagens TCP/JSON
└── README.md                # Esta documentação
```

---

## ⚙️ Pré-requisitos

- **Go 1.27** ou superior instalado (`go version`)
- **Docker e Docker Compose** (opcional, para execução em contêineres)
- Terminal (PowerShell, CMD, Bash)

---

## 🚀 Como Executar

### Cenário 1: Execução Local Rápida via Docker Compose (Mesma Máquina)

O `docker-compose.yaml` sobe o consórcio com as 3 empresas em contêineres isolados, cada um com volume de disco próprio:

```bash
# 1. Constrói as imagens e sobe as 3 empresas em segundo plano
docker compose -f docker/docker-compose.yaml up -d --build

# 2. Povoa a Empresa A com caronas de demonstração
docker compose -f docker/docker-compose.yaml run --rm seed

# 3. Abre um cliente interativo de terminal conectado à Empresa A
docker compose -f docker/docker-compose.yaml run --rm client

# Para conectar o cliente na Empresa B:
docker compose -f docker/docker-compose.yaml run --rm -e VAIJUNTO_SERVER_ADDR=empresa-b:8080 client
```

Portas publicadas na máquina host:
- **Empresa A:** TCP `8080` | REST `8081`
- **Empresa B:** TCP `8082` | REST `8083`
- **Empresa C:** TCP `8084` | REST `8085`

Para parar e remover os contêineres:
```bash
docker compose -f docker/docker-compose.yaml down
```

---

### Cenário 2: Execução em 3 Computadores Físicos Distintos (Apresentação)

Na apresentação com 3 computadores na mesma rede Wi-Fi ou cabo de rede local (LAN):

1. Descubra o IP local de cada computador (via `ipconfig` no Windows ou `ip a` no Linux):
   - **PC 1 (Empresa A):** ex: `192.168.1.10`
   - **PC 2 (Empresa B):** ex: `192.168.1.20`
   - **PC 3 (Empresa C):** ex: `192.168.1.30`

#### Opção A: Executando Diretamente com Go Nativo

**💻 No Computador 1 (Empresa A):**
```bash
go run ./cmd/api-server -company-id empresa-a -tcp-addr 0.0.0.0:8080 -http-addr 0.0.0.0:8081 -peers "empresa-b=http://192.168.1.20:8081,empresa-c=http://192.168.1.30:8081"
```

**💻 No Computador 2 (Empresa B):**
```bash
go run ./cmd/api-server -company-id empresa-b -tcp-addr 0.0.0.0:8080 -http-addr 0.0.0.0:8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-c=http://192.168.1.30:8081"
```

**💻 No Computador 3 (Empresa C):**
```bash
go run ./cmd/api-server -company-id empresa-c -tcp-addr 0.0.0.0:8080 -http-addr 0.0.0.0:8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-b=http://192.168.1.20:8081"
```

#### Opção B: Executando com Docker em Cada Computador

Primeiro construa a imagem em cada PC:
```bash
docker build -t vaijunto:local -f docker/Dockerfile .
```

Em seguida, suba o contêiner em cada máquina:

**💻 No Computador 1 (Empresa A):**
```bash
docker run --name empresa-a --rm -p 8080:8080 -p 8081:8081 vaijunto:local /app/bin/api-server -company-id empresa-a -tcp-addr :8080 -http-addr :8081 -peers "empresa-b=http://192.168.1.20:8081,empresa-c=http://192.168.1.30:8081"
```

**💻 No Computador 2 (Empresa B):**
```bash
docker run --name empresa-b --rm -p 8080:8080 -p 8081:8081 vaijunto:local /app/bin/api-server -company-id empresa-b -tcp-addr :8080 -http-addr :8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-c=http://192.168.1.30:8081"
```

**💻 No Computador 3 (Empresa C):**
```bash
docker run --name empresa-c --rm -p 8080:8080 -p 8081:8081 vaijunto:local /app/bin/api-server -company-id empresa-c -tcp-addr :8080 -http-addr :8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-b=http://192.168.1.20:8081"
```

---

### Conectando os Clientes de Terminal

Em qualquer computador da rede, inicie o cliente de terminal apontando para o IP do servidor desejado:

```bash
# Conectar como passageiro na Empresa A:
go run ./cmd/api-client -addr 192.168.1.10:8080

# Conectar como passageiro na Empresa B:
go run ./cmd/api-client -addr 192.168.1.20:8080
```

---

## ⚡ Simulador de Alta Carga Concorrente

O projeto inclui um simulador autônomo de estresse (`cmd/simulator`) que gera dezenas de passageiros simultâneos concorrendo por vagas e medindo throughput e rejeições:

```bash
# Executando nativamente contra o consórcio:
go run ./cmd/simulator -companies="empresa-a=localhost:8080,empresa-b=localhost:8082,empresa-c=localhost:8084" -passengers=30 -duration=15s

# Ou via Docker Compose:
docker compose -f docker/docker-compose.yaml run --rm simulator
```

---

## 🧪 Testes Automatizados e Injeção de Falhas

O sistema conta com testes unitários em todos os pacotes e uma suíte completa de **testes de integração com injeção de falhas** (`tests/integration/`):

Execute toda a suíte de testes do projeto:
```bash
go test ./... -v
```

### Propriedades Distribuídas Provadas nos Testes:

1. **`TestNoDoubleSeatBooking`:** 21 clientes simultâneos distribuídos entre as 3 empresas disputam um único assento remanescente. O teste valida que **estritamente 1 cliente tem a reserva confirmada e 20 são rejeitados**, provando a ausência de reservas duplicadas.
2. **`TestNoPartialItinerary`:** Em um itinerário multitrecho inter-empresas, simula a queda do servidor participante durante o `PREPARE` e rejeições de vagas. Comprova que **nenhum assento é consumido de forma parcial (Rollback Atômico total)**.
3. **`TestNoBlockedSeatsAfterRecovery`:** Simula um participante que recebe um `PREPARE` e cai antes de receber o `COMMIT`. Após a expiração do TTL (10s), o servidor é religado e o teste comprova que o assento foi desbloqueado e pode ser reservado normalmente por novos passageiros.

---

## 📡 Referência da API REST Inter-Empresas (Gin)

| Método | Endpoint | Descrição |
|---|---|---|
| `GET` | `/internal/v1/health` | Healthcheck do nó e valor atual do Lamport Clock |
| `GET` | `/internal/v1/rides?date=YYYY-MM-DD` | Consulta federada de caronas ativas por data |
| `POST` | `/internal/v1/segments/:segmentID/prepare` | Fase 1 do 2PC: valida assento e adquire lock com TTL |
| `POST` | `/internal/v1/segments/:segmentID/commit` | Fase 2 do 2PC: efetiva decremento e libera lock |
| `POST` | `/internal/v1/segments/:segmentID/abort` | Fase 2 do 2PC: aborta transação e libera lock |
| `GET` | `/internal/v1/segments/:segmentID/status/:txID`| Consulta estado da transação para recuperação pós-queda |

---

## 🛡️ Conceitos-Chave para Apresentação

- **Consenso sem Coordenador Fixo:** Qualquer empresa que receba o pedido do passageiro atua como coordenadora daquela transação 2PC específica.
- **Ordem Total Determinística:** O desempate em disputas concorrentes de assentos utiliza o par `(LamportClock, CoordinatorCompanyID)`. Como o ID da empresa é uma string única, nunca há empate sem vencedor.
- **Ponto de Não-Retorno:** A decisão de `COMMIT` é persistida no WAL pelo coordenador com `fsync` antes de enviar mensagens aos participantes. Em caso de queda, a transação é recuperada e finalizada no boot.
- **Prevenção do Blocking Problem:** Locks temporizados por TTL nos participantes impedem que falhas permanentes de coordenadores travem assentos indefinidamente.
