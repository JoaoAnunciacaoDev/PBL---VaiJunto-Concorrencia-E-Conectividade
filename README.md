# VaiJunto — Consórcio Distribuído de Caronas Compartilhadas

Aplicação de caronas compartilhadas distribuída construída em Go para a disciplina de **Concorrência e Conectividade (PBL 2)**. 

O sistema implementa um **consórcio descentralizado** formado por múltiplas empresas independentes (`empresa-a`, `empresa-b`, `empresa-c`), onde passageiros podem buscar e reservar itinerários intermunicipais compostos por trechos operados por diferentes empresas parceiras, **sem qualquer banco de dados compartilhado, sem coordenador fixo central e sem sistemas de terceiros (sem Redis, ZooKeeper, etcd ou Consul)**.

A consistência e a atomicidade das reservas entre os múltiplos servidores são garantidas através de uma implementação própria de **Two-Phase Commit (2PC)**, **Relógios Lógicos de Lamport**, **Locks temporizados por TTL** e **Write-Ahead Log (WAL)** com sincronização síncrona em disco (`fsync`).

---

## Arquitetura Distribuída

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

## Estrutura do Projeto

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
├── PROTOCOL.md              # Documentação das mensagens TCP/JSON (Clientes)
├── REST_API.md              # Documentação da API REST Inter-Servidores (2PC)
└── README.md                # Esta documentação
```

---

## Pré-requisitos

- **Go 1.27** ou superior instalado (`go version`)
- **Docker e Docker Compose** (opcional, para execução em contêineres)
- Terminal (PowerShell, CMD, Bash)

---

## Como Executar

### Cenário 1: Execução Local Rápida via Docker Compose (Mesma Máquina)

O `docker-compose.yaml` orquestra o consórcio com as 3 empresas em contêineres isolados, cada um com volume de disco próprio, e roda automaticamente o serviço de `seed` assim que todos estiverem saudáveis:

```bash
# 1. Constrói as imagens, sobe as 3 empresas e executa o seed multiconsórcio automaticamente
docker compose -f docker/docker-compose.yaml up -d --build

# 2. Acompanhar os logs dos servidores ativos em tempo real (coloridos por empresa):
docker compose -f docker/docker-compose.yaml logs -f empresa-a empresa-b empresa-c

# (Ou ver logs de uma empresa específica: logs -f empresa-a)

# 3. Abre um cliente interativo de terminal conectado à Empresa A
docker compose -f docker/docker-compose.yaml run --rm client

# Para conectar o cliente na Empresa B ou C (direto via argumento ou via -e):
docker compose -f docker/docker-compose.yaml run --rm client -addr empresa-b:8080
docker compose -f docker/docker-compose.yaml run --rm client -addr empresa-c:8080
# (Ou via variável de ambiente: run --rm -e VAIJUNTO_SERVER_ADDR=empresa-b:8080 client)
```

> **Nota sobre o Cliente Interativo:** O serviço `client` utiliza o perfil `profiles: ["tools"]`, o que significa que o comando `docker compose up -d` sobe apenas os servidores e o seed em segundo plano. Para interagir com o terminal do cliente, utilize sempre `docker compose run --rm client`, conectando teclado e tela diretamente à sessão.

Portas publicadas na máquina host:
- **Empresa A:** TCP `8080` | REST `8081`
- **Empresa B:** TCP `8082` | REST `8083`
- **Empresa C:** TCP `8084` | REST `8085`

#### Monitoramento de Logs no Docker Compose:
```bash
# Acompanhar logs de todos os 3 servidores simultaneamente em tempo real:
docker compose -f docker/docker-compose.yaml logs -f empresa-a empresa-b empresa-c

# Acompanhar logs de um servidor específico:
docker compose -f docker/docker-compose.yaml logs -f empresa-a

# Consultar as últimas 50 linhas do log sem manter o processo anexado:
docker compose -f docker/docker-compose.yaml logs --tail 50 empresa-a
```

Para parar e remover os contêineres:
```bash
docker compose -f docker/docker-compose.yaml down
```

---

### Catálogo de Viagens e Dados Pré-configurados (Seed)

> **Disponibilidade Temporal dos Dados:**  
> Por padrão, o utilitário `cmd/seed` cria registros para a data corrente (`D`) e para a data subsequente (`D+1`), no formato `dd/mm/aaaa`. Essa estratégia assegura a disponibilidade contínua de itinerários para consultas e testes independentemente do dia em que o ambiente for instanciado.

#### 1. Catálogo de Caronas por Empresa

##### Empresa A (Servidor TCP: `:8080` | REST: `:8081`)
* **Motorista:** Ana (`ana@empresa-a.com`) | Veículo: *Toyota Corolla Branco (Placa: ANA-1A01)* | Capacidade: 4 vagas
  * **Carona 1 (Itinerário com múltiplos trechos):**
    * Trecho 1: `Salvador` (07:00) ➔ `Feira de Santana` (08:30) — R$ 35,00
    * Trecho 2: `Feira de Santana` (08:45) ➔ `Alagoinhas` (09:45) — R$ 20,00
  * **Carona 2 (Rota Concorrente):**
    * Trecho Único: `Feira de Santana` (08:00) ➔ `Salvador` (09:30) — R$ 35,00

##### Empresa B (Servidor TCP: `:8082` | REST: `:8083`)
* **Motorista:** Bruno (`bruno@empresa-b.com`) | Veículo: *Honda Civic Prata (Placa: BRU-2B02)* | Capacidade: 4 vagas
  * **Carona 1 (Trecho Intermediário para Chapada Diamantina):**
    * Trecho Único: `Feira de Santana` (09:00) ➔ `Lençóis` (13:00) — R$ 65,00  
      *(Permite conexão com a Carona 1 da Empresa A, cuja chegada em Feira de Santana ocorre às 08:30)*
  * **Carona 2 (Rota Concorrente):**
    * Trecho Único: `Feira de Santana` (08:15) ➔ `Salvador` (09:45) — R$ 30,00

##### Empresa C (Servidor TCP: `:8084` | REST: `:8085`)
* **Motorista:** Carlos (`carlos@empresa-c.com`) | Veículo: *Jeep Renegade Preto (Placa: CAR-3C03)* | Capacidade: 4 vagas
  * **Carona 1 (Trecho Final para Região Sudoeste):**
    * Trecho Único: `Lençóis` (14:00) ➔ `Vitória da Conquista` (18:00) — R$ 55,00  
      *(Permite conexão com a Carona 1 da Empresa B, cuja chegada em Lençóis ocorre às 13:00)*
  * **Carona 2 (Rota Concorrente):**
    * Trecho Único: `Feira de Santana` (08:30) ➔ `Salvador` (10:00) — R$ 25,00

---

#### 2. Cenários de Validação do Consórcio

* **Cenário A — Consulta e Comparação de Tarifas:**
  * **Rota consultada:** `Feira de Santana` ➔ `Salvador`
  * **Comportamento:** O servidor consultado agrega ofertas de todos os nós ativos do consórcio, retornando três opções concorrentes:
    1. Empresa C (Carlos): R$ 25,00 (08:30)
    2. Empresa B (Bruno): R$ 30,00 (08:15)
    3. Empresa A (Ana): R$ 35,00 (08:00)

* **Cenário B — Transação Atômica Distribuída entre 2 Empresas (2PC):**
  * **Rota consultada:** `Salvador` ➔ `Lençóis`
  * **Composição:** Trecho 1 na **Empresa A** (07:00) + Trecho 2 na **Empresa B** (09:00).
  * **Garantia:** Coordenação atômica Two-Phase Commit entre os servidores da Empresa A (coordenador) e da Empresa B (participante).

* **Cenário C — Transação Atômica Triangular entre 3 Empresas (2PC):**
  * **Rota consultada:** `Salvador` ➔ `Vitória da Conquista`
  * **Composição:** Trecho 1 na **Empresa A** (SSA ➔ FSA) + Trecho 2 na **Empresa B** (FSA ➔ Lençóis) + Trecho 3 na **Empresa C** (Lençóis ➔ Conquista).
  * **Garantia:** Coordenação do commit em duas fases envolvendo simultaneamente os três nós independentes do consórcio.

---

#### 3. Usuários e Senhas Pré-cadastrados para Login:
> **Senha padrão para todas as contas:** `12345678Abc#`

| Empresa | Papel | Nome | E-mail de Login | Senha |
|---|---|---|---|---|
| **Empresa A** | Passageira | Alice Passageira | `alice@gmail.com` | `12345678Abc#` |
| **Empresa A** | Motorista | Ana Motorista | `ana@empresa-a.com` | `12345678Abc#` |
| **Empresa B** | Passageiro | Beto Passageiro | `beto@gmail.com` | `12345678Abc#` |
| **Empresa B** | Motorista | Bruno Motorista | `bruno@empresa-b.com` | `12345678Abc#` |
| **Empresa C** | Passageira | Carla Passageira | `carla@gmail.com` | `12345678Abc#` |
| **Empresa C** | Motorista | Carlos Motorista | `carlos@empresa-c.com` | `12345678Abc#` |

---

### Cenário 2: Execução Distribuída em Múltiplos Computadores Físicos

Para implantação do consórcio em computadores distintos interconectados via rede local (LAN ou Wi-Fi):

1. Identifique o endereço IP local de cada computador (`ip a` ou `hostname -I` no Linux; `ipconfig` no Windows):
   - **PC 1 (Empresa A):** Exemplo: `192.168.1.10`
   - **PC 2 (Empresa B):** Exemplo: `192.168.1.20`
   - **PC 3 (Empresa C):** Exemplo: `192.168.1.30`

#### Opção A: Execução Nativa em Go

**No Computador 1 (Empresa A):**
```bash
go run ./cmd/api-server -company-id empresa-a -tcp-addr 0.0.0.0:8080 -http-addr 0.0.0.0:8081 -peers "empresa-b=http://192.168.1.20:8081,empresa-c=http://192.168.1.30:8081"
```

**No Computador 2 (Empresa B):**
```bash
go run ./cmd/api-server -company-id empresa-b -tcp-addr 0.0.0.0:8080 -http-addr 0.0.0.0:8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-c=http://192.168.1.30:8081"
```

**No Computador 3 (Empresa C):**
```bash
go run ./cmd/api-server -company-id empresa-c -tcp-addr 0.0.0.0:8080 -http-addr 0.0.0.0:8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-b=http://192.168.1.20:8081"
```

#### Opção B: Execução com Contêineres Docker Individuais

Neste modelo, cada estação executa exclusivamente o contêiner do seu respectivo nó, expondo as portas `8080` (clientes) e `8081` (comunicação inter-nós):

1. Construção da imagem Docker em cada estação:
```bash
docker build --no-cache -t vaijunto:local -f docker/Dockerfile .
```

2. Inicialização dos serviços em cada estação:

**No Computador 1 (Empresa A):**
```bash
docker rm -f empresa-a 2>/dev/null

docker run -d \
  --name empresa-a \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 8081:8081 \
  -v empresa-a-data:/app/storage \
  vaijunto:local \
  /app/bin/api-server \
  -company-id empresa-a \
  -data-dir /app/storage/data \
  -tcp-addr :8080 \
  -http-addr :8081 \
  -peers "empresa-b=http://172.16.103.12:8081,empresa-c=http://172.16.103.10:8081"
```

**No Computador 2 (Empresa B):**
```bash
docker rm -f empresa-b 2>/dev/null

docker run -d \
  --name empresa-b \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 8081:8081 \
  -v empresa-b-data:/app/storage \
  vaijunto:local \
  /app/bin/api-server \
  -company-id empresa-b \
  -data-dir /app/storage/data \
  -tcp-addr :8080 \
  -http-addr :8081 \
  -peers "empresa-a=http://172.16.103.11:8081,empresa-c=http://172.16.103.10:8081"
```

**No Computador 3 (Empresa C):**
```bash
docker rm -f empresa-c 2>/dev/null

docker run -d \
  --name empresa-c \
  --restart unless-stopped \
  -p 8080:8080 \
  -p 8081:8081 \
  -v empresa-c-data:/app/storage \
  vaijunto:local \
  /app/bin/api-server \
  -company-id empresa-c \
  -data-dir /app/storage/data \
  -tcp-addr :8080 \
  -http-addr :8081 \
  -peers "empresa-a=http://172.16.103.11:8081,empresa-b=http://172.16.103.12:8081"
```

#### Monitoramento de Logs por Nó:
```bash
# No terminal da Empresa A:
docker logs -f empresa-a

# No terminal da Empresa B:
docker logs -f empresa-b

# No terminal da Empresa C:
docker logs -f empresa-c
```
*(Durante uma transação distribuída, o nó coordenador registrará os estágios do 2PC enquanto o nó participante registrará o processamento HTTP nos endpoints de pré-reserva e commit).*

---

### Inicialização dos Dados no Consórcio Distribuído

Após a inicialização dos servidores nas três estações, execute o povoamento inicial a partir de **qualquer computador** da rede (operação única para todo o consórcio):

```bash
# Execução via Go:
go run ./cmd/seed -servers="empresa-a=192.168.1.10:8080,empresa-b=192.168.1.20:8080,empresa-c=192.168.1.30:8080"

# Execução via Docker:
docker run --rm vaijunto:local /app/bin/seed -servers="empresa-a=172.16.103.11:8080,empresa-b=172.16.103.12:8080,empresa-c=172.16.103.10:8080"
```

---

### Execução dos Clientes de Terminal

O cliente interativo de terminal pode ser executado em qualquer computador conectado à rede:

#### 1. Execução via Go Nativo:
```bash
# Conexão à Empresa A:
go run ./cmd/api-client -addr 192.168.1.10:8080

# Conexão à Empresa B:
go run ./cmd/api-client -addr 192.168.1.20:8080

# Conexão à Empresa C:
go run ./cmd/api-client -addr 192.168.1.30:8080
```

#### 2. Execução via Contêiner Docker (Interativo):
```bash
# Conexão à Empresa A:
docker run -it --rm vaijunto:local /app/bin/api-client -addr 192.168.1.10:8080

# Conexão à Empresa B:
docker run -it --rm vaijunto:local /app/bin/api-client -addr 192.168.1.20:8080

# Conexão à Empresa C:
docker run -it --rm vaijunto:local /app/bin/api-client -addr 192.168.1.30:8080
```

---

## Simulador de Alta Carga Concorrente

O projeto inclui um simulador autônomo de estresse (`cmd/simulator`) que gera dezenas de passageiros simultâneos concorrendo por vagas e medindo throughput e rejeições:

```bash
# Execução direta via Docker Compose (sobe os nós, roda o seed e executa o teste de estresse):
docker compose -f docker/docker-compose.yaml run --build --rm simulator

# Para resetar os assentos e rodar novamente do zero:
docker compose -f docker/docker-compose.yaml down -v
docker compose -f docker/docker-compose.yaml run --build --rm simulator

# Ou executando nativamente em Go contra nós locais já ligados:
go run ./cmd/simulator -companies="empresa-a=localhost:8080,empresa-b=localhost:8082,empresa-c=localhost:8084" -passengers=30 -duration=15s
```

---

## Testes Automatizados e Injeção de Falhas

O sistema conta com testes unitários em todos os pacotes e uma suíte completa de **testes de integração com injeção de falhas** (`tests/integration/`):

Execute toda a suíte de testes do projeto:
```bash
# Executando nativamente com Go:
go test ./... -v

# Ou executando toda a suíte de testes via Docker (sem necessidade de Go instalado na máquina):
docker run --rm -v ${PWD}:/app -w /app golang:1.27 go test ./... -v
```

Executar um teste específico (ex.: teste de reserva concorrente ou teste de recuperação):
```bash
# Executando nativamente com Go:
go test -v -run TestNoDoubleSeatBooking ./tests/integration
go test -v -run TestDistributedCancellationAndDuplicateCheck ./tests/integration

# Ou executando o teste específico via Docker:
docker run --rm -v ${PWD}:/app -w /app golang:1.27 go test -v -run TestNoDoubleSeatBooking ./tests/integration
```

Executar os testes de um pacote específico (ex.: consortium ou httpapi):
```bash
# Executando nativamente com Go:
go test -v ./internal/consortium/...

# Ou via Docker:
docker run --rm -v ${PWD}:/app -w /app golang:1.27 go test -v ./internal/consortium/...
```

### Propriedades Distribuídas Provadas nos Testes:

1. **`TestNoDoubleSeatBooking`:** 21 clientes simultâneos distribuídos entre as 3 empresas disputam um único assento remanescente. O teste valida que **estritamente 1 cliente tem a reserva confirmada e 20 são rejeitados**, provando a ausência de reservas duplicadas.
2. **`TestNoPartialItinerary`:** Em um itinerário multitrecho inter-empresas, simula a queda do servidor participante durante o `PREPARE` e rejeições de vagas. Comprova que **nenhum assento é consumido de forma parcial (Rollback Atômico total)**.
3. **`TestNoBlockedSeatsAfterRecovery`:** Simula um participante que recebe um `PREPARE` e cai antes de receber o `COMMIT`. Após a expiração do TTL (10s), o servidor é religado e o teste comprova que o assento foi desbloqueado e pode ser reservado normalmente por novos passageiros.

---

## Referência da API REST Inter-Empresas (Gin)

| Método | Endpoint | Descrição |
|---|---|---|
| `GET` | `/internal/v1/health` | Healthcheck do nó e valor atual do Lamport Clock |
| `GET` | `/internal/v1/rides?date=YYYY-MM-DD` | Consulta de caronas ativas por data |
| `POST` | `/internal/v1/segments/:segmentID/prepare` | Fase 1 do 2PC: valida assento e adquire lock com TTL |
| `POST` | `/internal/v1/segments/:segmentID/commit` | Fase 2 do 2PC: efetiva decremento e libera lock |
| `POST` | `/internal/v1/segments/:segmentID/abort` | Fase 2 do 2PC: aborta transação e libera lock |
| `GET` | `/internal/v1/segments/:segmentID/status/:txID`| Consulta estado da transação para recuperação pós-queda |

---

## Fundamentos Teóricos e Garantias de Consistência

- **Consenso sem Coordenador Fixo:** Qualquer nó do consórcio que recebe a solicitação de reserva atua como coordenador autônomo para aquela transação específica.
- **Ordem Total Determinística:** A resolução de conflitos concorrentes por assentos utiliza a tupla `(LamportClock, CoordinatorCompanyID)`. Como o identificador da empresa é único no consórcio, assegura-se desempate determinístico sem risco de impasses (*deadlocks*).
- **Ponto de Não-Retorno:** A decisão de `COMMIT` é registrada no Write-Ahead Log (WAL) com chamada síncrona `fsync` antes do envio das confirmações aos participantes. Em caso de falha de energia, o estado é reconstruído durante a inicialização do nó.
- **Prevenção do Blocking Problem:** Locks temporizados por TTL nos nós participantes impedem que indisponibilidades prolongadas de nós coordenadores causem bloqueio permanente de vagas.
