# Especificação da API REST Inter-Servidores (Consórcio VaiJunto)

Este documento especifica a **API REST HTTP** utilizada para a comunicação entre servidores parceiros do consórcio descentralizado VaiJunto.

A API é implementada sobre o framework **Gin Gonic** em Go e opera por padrão na porta TCP `8081` (configurável via flag `-http-addr` ou variável de ambiente `HTTP_ADDR`), isolada da porta de clientes finais de terminal (TCP `8080`).

---

## 1. Visão Geral dos Endpoints

Todos os endpoints utilizam o prefixo de rota `/internal/v1`.

| Método | Endpoint | Papel no Consórcio | Descrição |
|---|---|---|---|
| `GET` | `/internal/v1/health` | Monitoramento | Healthcheck do nó e leitura do relógio de Lamport |
| `GET` | `/internal/v1/rides` | Busca | Consulta caronas ativas da empresa para uma data |
| `GET` | `/internal/v1/rides/:rideID` | Consulta Direta | Consulta dados de uma carona específica por ID |
| `POST` | `/internal/v1/segments/:segmentID/prepare` | 2PC — Fase 1 | Pré-reserva temporizada e voto afirmativo/veto |
| `POST` | `/internal/v1/segments/:segmentID/commit` | 2PC — Fase 2 | Confirmação da reserva e decremento do assento |
| `POST` | `/internal/v1/segments/:segmentID/abort` | 2PC — Fase 2 | Descarte da pré-reserva em caso de falha global |
| `GET` | `/internal/v1/segments/:segmentID/status/:txID` | Recuperação Pós-Queda | Consulta o estado da transação no WAL ou LockStore |
| `POST` | `/internal/v1/segments/:segmentID/release` | Cancelamento Descentralizado | Libera assento previamente confirmado |

---

## 2. Detalhamento dos Endpoints

---

### 2.1. Healthcheck e Relógio Lógico

#### `GET /internal/v1/health`

Verifica a disponibilidade operacional do nó e retorna o estado atual do seu relógio lógico de Lamport. Utilizado por peers antes de despachar transações e pelos healthchecks do Docker Compose.

* **Parâmetros de Cabeçalho**: Nenhum.
* **Corpo da Requisição**: Vazio.

##### Resposta de Sucesso: `200 OK`
```json
{
  "company_id": "empresa-a",
  "status": "ok",
  "lamport_clock": 42
}
```

##### Exemplo de Chamada via `curl`:
```bash
curl -X GET http://localhost:8081/internal/v1/health
```

---

### 2.2. Busca de Caronas por Data

#### `GET /internal/v1/rides?date={YYYY-MM-DD}`

Permite que nós coordenadores consultem remotamente as caronas ativas de uma empresa parceira para compor itinerários multitrecho no algoritmo de busca.

* **Query Parameters**:
  * `date` (obrigatório, string): Data da viagem no formato `YYYY-MM-DD` ou `RFC3339`.

##### Resposta de Sucesso: `200 OK`
```json
[
  {
    "id": "e2f3a4b5-c6d7-8901-bcde-f12345678901",
    "driver_id": "11223344-5566-7788-99aa-bbccddeeff00",
    "driver_name": "Ana Motorista",
    "company_id": "empresa-a",
    "departure_at": "2026-11-04T08:00:00-03:00",
    "cancelled": false,
    "segments": [
      {
        "id": "c1f7b029-7928-48b4-9273-df5e6e3f0190",
        "origin": 1,
        "destination": 3,
        "departure_at": "2026-11-04T08:00:00-03:00",
        "arrival_at": "2026-11-04T09:30:00-03:00",
        "price_cents": 3500,
        "available_seats": 4
      }
    ]
  }
]
```

##### Respostas de Erro:
* **`400 Bad Request`**: Parâmetro `date` ausente ou em formato inválido.
  ```json
  { "error": "parâmetro 'date' é obrigatório (formato YYYY-MM-DD)" }
  ```
* **`500 Internal Server Error`**: Falha na leitura do repositório local.
  ```json
  { "error": "não foi possível buscar caronas" }
  ```

##### Exemplo de Chamada via `curl`:
```bash
curl -X GET "http://localhost:8081/internal/v1/rides?date=2026-11-04"
```

---

### 2.3. Consulta Direta de Carona por ID

#### `GET /internal/v1/rides/:rideID`

Retorna os dados de uma carona específica da empresa parceira, utilizado na reconstrução de itinerários durante a confirmação de reservas.

* **Path Parameters**:
  * `rideID` (obrigatório, UUID): Identificador único da carona.

##### Resposta de Sucesso: `200 OK`
```json
{
  "id": "e2f3a4b5-c6d7-8901-bcde-f12345678901",
  "driver_id": "11223344-5566-7788-99aa-bbccddeeff00",
  "driver_name": "Ana Motorista",
  "company_id": "empresa-a",
  "departure_at": "2026-11-04T08:00:00-03:00",
  "cancelled": false,
  "segments": [
    {
      "id": "c1f7b029-7928-48b4-9273-df5e6e3f0190",
      "origin": 1,
      "destination": 3,
      "departure_at": "2026-11-04T08:00:00-03:00",
      "arrival_at": "2026-11-04T09:30:00-03:00",
      "price_cents": 3500,
      "available_seats": 4
    }
  ]
}
```

##### Respostas de Erro:
* **`400 Bad Request`**: Formato do UUID inválido.
  ```json
  { "error": "ID de carona inválido" }
  ```
* **`404 Not Found`**: Carona inexistente ou não encontrada no repositório.
  ```json
  { "error": "carona não encontrada" }
  ```

---

### 2.4. 2PC Fase 1: Prepare

#### `POST /internal/v1/segments/:segmentID/prepare`

Requisita a pré-reserva temporizada de um trecho da carona. O nó participante:
1. Sincroniza seu relógio de Lamport (`s.clock.Update`).
2. Verifica se a carona existe e possui assentos disponíveis (`available_seats > 0`).
3. Tenta adquirir a trava no `LockStore` via resolução de precedência lógica.
4. Marca o lock como preparado (`MarkPrepared`).
5. Grava duravelmente o registro `TxPrepared` no Write-Ahead Log (WAL) com chamada síncrona `fsync`. Se a gravação em disco falhar, o voto afirmativo é abortado e a trava é liberada.

* **Path Parameters**:
  * `segmentID` (obrigatório, UUID): Identificador único do trecho da carona.

##### Corpo da Requisição (`application/json`):
```json
{
  "tx_id": "a89b7c6d-1234-5678-9abc-def012345678",
  "coordinator_company_id": "empresa-a",
  "lamport_clock": 14,
  "passenger_id": "b1c2d3e4-f5a6-7890-1234-567890abcdef",
  "ride_id": "e2f3a4b5-c6d7-8901-bcde-f12345678901"
}
```

##### Resposta de Voto Afirmativo: `200 OK`
```json
{
  "ok": true,
  "lamport_clock": 15
}
```

##### Resposta de Voto Negativo (Conflito): `409 Conflict`
Ocorre quando não há vagas no trecho ou quando outro lock com maior prioridade de Lamport já ocupa o assento.
```json
{
  "ok": false,
  "lamport_clock": 15,
  "reason": "seat_unavailable"
}
```
*Valores possíveis para `reason`*:
* `seat_unavailable`: Assento já esgotado no trecho físico.
* `ride_unavailable`: Carona inexistente ou cancelada.
* `lock_conflict`: Conflito de precedência com outra transação ativa.

##### Respostas de Erro de Execução:
* **`400 Bad Request`**: UUID inválido ou JSON mal formatado.
* **`500 Internal Server Error`**: Falha ao persistir entrada `TxPrepared` no WAL com `fsync` (`wal_write_failed`).

##### Exemplo de Chamada via `curl`:
```bash
curl -X POST http://localhost:8081/internal/v1/segments/c1f7b029-7928-48b4-9273-df5e6e3f0190/prepare \
  -H "Content-Type: application/json" \
  -d '{
    "tx_id": "a89b7c6d-1234-5678-9abc-def012345678",
    "coordinator_company_id": "empresa-a",
    "lamport_clock": 14,
    "passenger_id": "b1c2d3e4-f5a6-7890-1234-567890abcdef",
    "ride_id": "e2f3a4b5-c6d7-8901-bcde-f12345678901"
  }'
```

---

### 2.5. 2PC Fase 2: Commit

#### `POST /internal/v1/segments/:segmentID/commit`

Enviado pelo Coordenador quando **todos** os participantes votaram afirmativamente no Prepare. O participante:
1. Avança seu relógio de Lamport.
2. Consulta o WAL. Se a transação já foi consolidada anteriormente (`TxCommitted`), responde `200 OK` garantindo idempotência. Se estiver abortada, retorna `409 Conflict`.
3. Valida se o lock ainda reside no estado preparado. Se o lock tiver sumido, responde `410 Gone`.
4. **Ponto de Não Retorno**: Grava a entrada durável `TxCommitted` no WAL com `fsync`.
5. Executa a mutação definitiva no repositório (`DecrementSeatIdempotent`), subtraindo o assento físico.
6. Libera a trava temporária da memória (`lockStore.Release`).

* **Path Parameters**:
  * `segmentID` (obrigatório, UUID): Identificador único do trecho.

##### Corpo da Requisição (`application/json`):
```json
{
  "tx_id": "a89b7c6d-1234-5678-9abc-def012345678",
  "coordinator_company_id": "empresa-a",
  "ride_id": "e2f3a4b5-c6d7-8901-bcde-f12345678901"
}
```

##### Resposta de Sucesso: `200 OK`
```json
{
  "ok": true
}
```

##### Respostas de Erro:
* **`409 Conflict`**: Transação marcada previamente como abortada (`transaction_aborted`).
* **`410 Gone`**: Lock preparado expirou ou não foi encontrado em memória/WAL (`lock_not_found`).
* **`400 Bad Request`**: Parâmetros incorretos ou `ride_id` ausente.
* **`500 Internal Server Error`**: Falha ao gravar no WAL (`wal_write_failed`) ou no repositório.

##### Exemplo de Chamada via `curl`:
```bash
curl -X POST http://localhost:8081/internal/v1/segments/c1f7b029-7928-48b4-9273-df5e6e3f0190/commit \
  -H "Content-Type: application/json" \
  -d '{
    "tx_id": "a89b7c6d-1234-5678-9abc-def012345678",
    "coordinator_company_id": "empresa-a",
    "ride_id": "e2f3a4b5-c6d7-8901-bcde-f12345678901"
  }'
```

---

### 2.6. 2PC Fase 2: Abort

#### `POST /internal/v1/segments/:segmentID/abort`

Enviado pelo Coordenador quando houve falha em qualquer um dos trechos (voto negativo, timeout ou queda de participante). Desfaz a pré-reserva.
1. Se a transação já constava como `TxCommitted` no WAL, a decisão de commit é definitiva e a mensagem de abort atrasada é ignorada de forma segura (preservando o assento).
2. Se já estava abortada, retorna `200 OK` de forma idempotente.
3. Libera o lock da memória e persiste `TxAborted` no WAL com `fsync`.

* **Path Parameters**:
  * `segmentID` (obrigatório, UUID): Identificador do trecho a abortar.

##### Corpo da Requisição (`application/json`):
```json
{
  "tx_id": "a89b7c6d-1234-5678-9abc-def012345678",
  "coordinator_company_id": "empresa-a"
}
```

##### Resposta de Sucesso: `200 OK`
```json
{
  "ok": true
}
```

##### Respostas de Erro:
* **`400 Bad Request`**: Payload inválido.
* **`500 Internal Server Error`**: Erro ao gravar descarte no WAL (`wal_write_failed`).

##### Exemplo de Chamada via `curl`:
```bash
curl -X POST http://localhost:8081/internal/v1/segments/c1f7b029-7928-48b4-9273-df5e6e3f0190/abort \
  -H "Content-Type: application/json" \
  -d '{
    "tx_id": "a89b7c6d-1234-5678-9abc-def012345678",
    "coordinator_company_id": "empresa-a"
  }'
```

---

### 2.7. Consulta de Estado de Transação (Recuperação)

#### `GET /internal/v1/segments/:segmentID/status/:txID`

Permite a resolução de incerteza distribuída quando um Coordenador reinicia após uma queda ou durante recuperação pós-partição de rede.

* **Path Parameters**:
  * `segmentID` (obrigatório, string/UUID): Identificador do trecho.
  * `txID` (obrigatório, string): Identificador da transação 2PC sob verificação.

##### Resposta de Sucesso: `200 OK`
```json
{
  "tx_id": "a89b7c6d-1234-5678-9abc-def012345678",
  "segment_id": "c1f7b029-7928-48b4-9273-df5e6e3f0190",
  "status": "COMMITTED"
}
```
*Estados possíveis*:
* `PREPARED`: O lock está ativo na memória ou anotado no WAL como preparado.
* `COMMITTED`: A transação foi confirmada definitivamente em disco.
* `ABORTED`: A transação foi cancelada.
* `NOT_FOUND`: Nenhum registro da transação foi localizado no nó.

##### Exemplo de Chamada via `curl`:
```bash
curl -X GET http://localhost:8081/internal/v1/segments/c1f7b029-7928-48b4-9273-df5e6e3f0190/status/a89b7c6d-1234-5678-9abc-def012345678
```

---

### 2.8. Cancelamento Descentralizado: Release

#### `POST /internal/v1/segments/:segmentID/release`

Utilizado exclusivamente quando um passageiro cancela uma reserva que já estava confirmada anteriormente. Quando o passageiro cancela uma viagem com múltiplos trechos na sua empresa de origem, o nó coordenador despacha requisições de `release` para as empresas parceiras que operavam trechos remotos.

O participante invoca `s.repo.IncrementSeat(rideID, segmentID)`, restaurando o assento físico e atualizando o relógio de Lamport.

* **Path Parameters**:
  * `segmentID` (obrigatório, UUID): Identificador do trecho cancelado.

##### Corpo da Requisição (`application/json`):
```json
{
  "ride_id": "e2f3a4b5-c6d7-8901-bcde-f12345678901",
  "coordinator_company_id": "empresa-a"
}
```
*(Nota: O campo `ride_id` é opcional caso o repositório consiga inferir a carona pelo trecho)*.

##### Resposta de Sucesso: `200 OK`
```json
{
  "ok": true
}
```

##### Respostas de Erro:
* **`400 Bad Request`**: UUID inválido (`segment_id_invalido`).
* **`404 Not Found`**: Carona ou trecho não localizado (`carona_nao_encontrada`).
* **`409 Conflict`**: Falha ao restaurar assento no repositório.

##### Exemplo de Chamada via `curl`:
```bash
curl -X POST http://localhost:8081/internal/v1/segments/c1f7b029-7928-48b4-9273-df5e6e3f0190/release \
  -H "Content-Type: application/json" \
  -d '{
    "ride_id": "e2f3a4b5-c6d7-8901-bcde-f12345678901",
    "coordinator_company_id": "empresa-a"
  }'
```

---

## 3. Tabela de Códigos de Status HTTP

| Código | Significado na Arquitetura Distribuída |
|---|---|
| `200 OK` | Operação concluída com sucesso (voto YES no prepare, commit/abort efetivado, release concluído). |
| `400 Bad Request` | Parâmetro de rota ou corpo JSON inválido/mal formatado. |
| `404 Not Found` | Recurso (carona ou trecho) inexistente no repositório do nó. |
| `409 Conflict` | Voto VETO no prepare (vagas esgotadas ou conflito de lock com transação concorrente) ou tentativa de commit de transação já abortada. |
| `410 Gone` | O lock temporizado expirou por TTL ou foi desalojado antes da chegada da decisão de commit. |
| `500 Internal Server Error` | Falha crítica de I/O em disco (ex.: falha de gravação com `fsync` no WAL). Impede votos afirmativos falsos. |
