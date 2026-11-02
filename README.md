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
├── PROTOCOL.md              # Documentação das mensagens TCP/JSON
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

#### 📋 Comandos Úteis para Logs no Docker Compose:
```bash
# Ver logs de todos os 3 servidores juntos em tempo real:
docker compose -f docker/docker-compose.yaml logs -f empresa-a empresa-b empresa-c

# Ver logs de um servidor específico:
docker compose -f docker/docker-compose.yaml logs -f empresa-a

# Ver apenas as últimas 50 linhas sem travar o terminal:
docker compose -f docker/docker-compose.yaml logs --tail 50 empresa-a
```

Para parar e remover os contêineres:
```bash
docker compose -f docker/docker-compose.yaml down
```

---

### 🌱 Catálogo de Viagens e Dados Pré-configurados (Seed)

> **Sobre as Datas das Viagens:**  
> Por padrão, o `cmd/seed` popula as caronas automaticamente para **HOJE** e para **AMANHÃ** (duas datas completas no formato `dd/mm/aaaa`). Assim, você pode testar hoje ou amanhã no laboratório digitando a data do próprio dia ou do dia seguinte — todas as viagens estarão ativas e prontas para busca!

#### 1. Catálogo Completo de Caronas por Empresa

##### 🏢 Empresa A (Servidor: `:8080` | REST: `:8081`)
* **Motorista:** Ana (`ana@empresa-a.com`) | Veículo: *Toyota Corolla Branco (Placa: ANA-1A01)* | Capacidade: 4 vagas
  * **Carona 1 (Multi-trecho conectável):**
    * Trecho 1: `Salvador` (07:00) ➔ `Feira de Santana` (08:30) — **R$ 35,00**
    * Trecho 2: `Feira de Santana` (08:45) ➔ `Alagoinhas` (09:45) — **R$ 20,00**
  * **Carona 2 (Concorrente direta):**
    * Trecho Único: `Feira de Santana` (08:00) ➔ `Salvador` (09:30) — **R$ 35,00**

##### 🏢 Empresa B (Servidor: `:8082` | REST: `:8083`)
* **Motorista:** Bruno (`bruno@empresa-b.com`) | Veículo: *Honda Civic Prata (Placa: BRU-2B02)* | Capacidade: 4 vagas
  * **Carona 1 (Conexão 2PC para Chapada):**
    * Trecho Único: `Feira de Santana` (09:00) ➔ `Lençóis` (13:00) — **R$ 65,00**  
      *(Conecta perfeitamente com a Carona 1 da Ana que chega a Feira às 08:30!)*
  * **Carona 2 (Concorrente direta mais barata):**
    * Trecho Único: `Feira de Santana` (08:15) ➔ `Salvador` (09:45) — **R$ 30,00**

##### 🏢 Empresa C (Servidor: `:8084` | REST: `:8085`)
* **Motorista:** Carlos (`carlos@empresa-c.com`) | Veículo: *Jeep Renegade Preto (Placa: CAR-3C03)* | Capacidade: 4 vagas
  * **Carona 1 (Conexão 2PC Sudoeste):**
    * Trecho Único: `Lençóis` (14:00) ➔ `Vitória da Conquista` (18:00) — **R$ 55,00**  
      *(Conecta com o Bruno que chega a Lençóis às 13:00!)*
  * **Carona 2 (Concorrente direta mais barata do consórcio):**
    * Trecho Único: `Feira de Santana` (08:30) ➔ `Salvador` (10:00) — **R$ 25,00**

---

#### 2. Cenários Prontos para Demonstração

* **Cenário A — Busca Federada com Concorrência:**
  * No cliente, busque: Origem `Feira de Santana` ➔ Destino `Salvador`
  * O consórcio retornará as **3 opções simultâneas** para comparação:
    1. Empresa C (Carlos): **R$ 25,00** (08:30)
    2. Empresa B (Bruno): **R$ 30,00** (08:15)
    3. Empresa A (Ana): **R$ 35,00** (08:00)

* **Cenário B — Two-Phase Commit entre 2 Empresas (SSA ➔ Lençóis):**
  * No cliente, busque: Origem `Salvador` ➔ Destino `Lençóis`
  * Itinerário composto por: Trecho 1 na **Empresa A** (07:00) + Trecho 2 na **Empresa B** (09:00).
  * Reserva atômica via 2PC com coordenação distribuída.

* **Cenário C — Two-Phase Commit Triangular entre 3 Empresas (SSA ➔ Conquista):**
  * No cliente, busque: Origem `Salvador` ➔ Destino `Vitória da Conquista`
  * Itinerário composto por 3 trechos: **Empresa A** (SSA➔FSA) + **Empresa B** (FSA➔Lençóis) + **Empresa C** (Lençóis➔Conquista).

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

### Cenário 2: Execução em 3 Computadores Físicos Distintos (Apresentação)

Na apresentação com 3 computadores na mesma rede Wi-Fi ou cabo de rede local (LAN):

1. Descubra o IP local de cada computador (via `ipconfig` no Windows ou `ip a` no Linux):
   - **PC 1 (Empresa A):** ex: `192.168.1.10`
   - **PC 2 (Empresa B):** ex: `192.168.1.20`
   - **PC 3 (Empresa C):** ex: `192.168.1.30`

#### Opção A: Executando Diretamente com Go Nativo

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

#### Opção B: Executando com Docker em Cada Computador

Neste cenário, cada computador roda **apenas o contêiner da sua própria empresa**. Como cada máquina publica as portas `8080` e `8081`, elas comunicam-se entre si pelo IP da rede local física.

1. Primeiro construa a imagem em cada PC:
```bash
docker build -t vaijunto:local -f docker/Dockerfile .
```

2. Em seguida, suba o contêiner em cada máquina (em segundo plano com `-d`):

**💻 No Computador 1 (Empresa A - IP `192.168.1.10`):**
```bash
docker run -d --name empresa-a --restart unless-stopped -p 8080:8080 -p 8081:8081 -v empresa-a-data:/app/storage vaijunto:local /app/bin/api-server -company-id empresa-a -tcp-addr :8080 -http-addr :8081 -peers "empresa-b=http://192.168.1.20:8081,empresa-c=http://192.168.1.30:8081"
```

**💻 No Computador 2 (Empresa B - IP `192.168.1.20`):**
```bash
docker run -d --name empresa-b --restart unless-stopped -p 8080:8080 -p 8081:8081 -v empresa-b-data:/app/storage vaijunto:local /app/bin/api-server -company-id empresa-b -tcp-addr :8080 -http-addr :8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-c=http://192.168.1.30:8081"
```

**💻 No Computador 3 (Empresa C - IP `192.168.1.30`):**
```bash
docker run -d --name empresa-c --restart unless-stopped -p 8080:8080 -p 8081:8081 -v empresa-c-data:/app/storage vaijunto:local /app/bin/api-server -company-id empresa-c -tcp-addr :8080 -http-addr :8081 -peers "empresa-a=http://192.168.1.10:8081,empresa-b=http://192.168.1.20:8081"
```

#### 📋 Como visualizar os logs de cada servidor:
Como cada servidor roda no seu respectivo computador, o log funciona diretamente no terminal daquela máquina:
```bash
# No terminal do PC 1:
docker logs -f empresa-a

# No terminal do PC 2:
docker logs -f empresa-b

# No terminal do PC 3:
docker logs -f empresa-c
```
*(Durante uma reserva com 2PC, você verá o terminal do PC 1 registrar o `[coordinator] PREPARE/COMMIT` e o terminal do PC 2 registrar o recebimento HTTP dos endpoints do participante).*

---

### Populando os 3 Computadores no Laboratório

Após ligar os servidores nas 3 máquinas, execute o `seed` a partir de **qualquer computador** da rede (basta rodar uma única vez):

```bash
# Usando Go nativo:
go run ./cmd/seed -servers="empresa-a=192.168.1.10:8080,empresa-b=192.168.1.20:8080,empresa-c=192.168.1.30:8080"

# Ou usando Docker (não precisa ter Go instalado):
docker run --rm vaijunto:local /app/bin/seed -servers="empresa-a=192.168.1.10:8080,empresa-b=192.168.1.20:8080,empresa-c=192.168.1.30:8080"
```

---

### Conectando os Clientes de Terminal

Qualquer computador pode abrir quantos clientes desejar (passageiros e motoristas). O cliente pode ser executado **nativamente em Go** ou **via Docker**:

#### 1. Usando Go nativo:
```bash
# Conectar à Empresa A (do próprio PC 1 use localhost ou o IP; de outro PC use o IP):
go run ./cmd/api-client -addr 192.168.1.10:8080

# Conectar à Empresa B:
go run ./cmd/api-client -addr 192.168.1.20:8080

# Conectar à Empresa C:
go run ./cmd/api-client -addr 192.168.1.30:8080
```

#### 2. Usando Docker (necessário `-it` para terminal interativo):
```bash
# Conectar à Empresa A:
docker run -it --rm vaijunto:local /app/bin/api-client -addr 192.168.1.10:8080

# Conectar à Empresa B:
docker run -it --rm vaijunto:local /app/bin/api-client -addr 192.168.1.20:8080

# Conectar à Empresa C:
docker run -it --rm vaijunto:local /app/bin/api-client -addr 192.168.1.30:8080
```

---

## Simulador de Alta Carga Concorrente

O projeto inclui um simulador autônomo de estresse (`cmd/simulator`) que gera dezenas de passageiros simultâneos concorrendo por vagas e medindo throughput e rejeições:

```bash
# Executando nativamente contra o consórcio:
go run ./cmd/simulator -companies="empresa-a=localhost:8080,empresa-b=localhost:8082,empresa-c=localhost:8084" -passengers=30 -duration=15s

# Ou via Docker Compose:
docker compose -f docker/docker-compose.yaml run --rm simulator
```

---

## Testes Automatizados e Injeção de Falhas

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

## Referência da API REST Inter-Empresas (Gin)

| Método | Endpoint | Descrição |
|---|---|---|
| `GET` | `/internal/v1/health` | Healthcheck do nó e valor atual do Lamport Clock |
| `GET` | `/internal/v1/rides?date=YYYY-MM-DD` | Consulta federada de caronas ativas por data |
| `POST` | `/internal/v1/segments/:segmentID/prepare` | Fase 1 do 2PC: valida assento e adquire lock com TTL |
| `POST` | `/internal/v1/segments/:segmentID/commit` | Fase 2 do 2PC: efetiva decremento e libera lock |
| `POST` | `/internal/v1/segments/:segmentID/abort` | Fase 2 do 2PC: aborta transação e libera lock |
| `GET` | `/internal/v1/segments/:segmentID/status/:txID`| Consulta estado da transação para recuperação pós-queda |

---

## Conceitos-Chave para Apresentação

- **Consenso sem Coordenador Fixo:** Qualquer empresa que receba o pedido do passageiro atua como coordenadora daquela transação 2PC específica.
- **Ordem Total Determinística:** O desempate em disputas concorrentes de assentos utiliza o par `(LamportClock, CoordinatorCompanyID)`. Como o ID da empresa é uma string única, nunca há empate sem vencedor.
- **Ponto de Não-Retorno:** A decisão de `COMMIT` é persistida no WAL pelo coordenador com `fsync` antes de enviar mensagens aos participantes. Em caso de queda, a transação é recuperada e finalizada no boot.
- **Prevenção do Blocking Problem:** Locks temporizados por TTL nos participantes impedem que falhas permanentes de coordenadores travem assentos indefinidamente.
