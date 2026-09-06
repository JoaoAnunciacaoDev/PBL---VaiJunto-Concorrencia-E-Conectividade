# Protocolo TCP/JSON do VaiJunto

Este documento descreve as mensagens aceitas pelo servidor do VaiJunto. Ele é a referência para os clientes conversarem com o servidor sem depender da implementação em Go.

## Transporte e enquadramento

- Transporte: TCP, por padrão na porta `8080`.
- Codificação: UTF-8.
- Formato: JSON.
- Cada objeto JSON é enviado como uma mensagem. O projeto usa `json.Encoder`, que termina cada objeto com quebra de linha (`\n`); portanto, envie um objeto por linha e leia uma resposta por linha.
- A conexão permanece aberta para várias requisições. As requisições de uma mesma conexão são processadas em sequência.
- Ao conectar, o servidor cria uma sessão anônima. O `login` associa um usuário àquela conexão; não existe token no JSON. Ao fechar a conexão ou executar `logout`, a sessão deixa de estar autenticada.

Datas e horários seguem o formato JSON padrão de `time.Time` do Go: RFC 3339, por exemplo `"2026-09-17T08:00:00Z"`.

## Envelope de requisição

Toda requisição possui `action` e `payload`:

```json
{
  "action": "login",
  "payload": {
    "email": "ana@example.com",
    "password": "Senha@123"
  }
}
```

Para ações sem corpo, envie `null` no payload:

```json
{"action":"logout","payload":null}
```

## Envelope de resposta

Toda resposta possui o mesmo formato:

```json
{
  "success": "success",
  "message": "Login bem-sucedido",
  "data": {}
}
```

| Campo | Tipo | Descrição |
| --- | --- | --- |
| `success` | string | `"success"` quando a ação foi concluída; `"error"` quando foi recusada. |
| `message` | string | Mensagem legível para exibir ao usuário. |
| `data` | objeto, lista ou ausente | Dados de retorno em respostas de sucesso. Não é enviado quando não há dados. |

Exemplo de erro:

```json
{
  "success": "error",
  "message": "Autenticação necessária."
}
```

## Valores enumerados

As cidades são números inteiros no JSON:

| Valor | Cidade |
| ---: | --- |
| `1` | Feira de Santana |
| `2` | Alagoinhas |
| `3` | Salvador |
| `4` | Lauro de Freitas |
| `5` | Camaçari |
| `6` | Vitória da Conquista |
| `7` | Jequié |
| `8` | Lençóis |

Os perfis são strings: `"PASSENGER"` e `"DRIVER"`.

O status de reserva é numérico: `1` = Pendente, `2` = Cancelada e `3` = Confirmada. Atualmente reservas criadas pelo sistema já recebem status `3`.

## Estruturas retornadas

Os UUIDs abaixo são strings, como `"1a2b3c4d-1111-2222-3333-123456789abc"`.

### Usuário (`UserResponse`)

```json
{
  "uuid": "<uuid>",
  "name": "Ana Souza",
  "email": "ana@example.com",
  "role": "PASSENGER"
}
```

### Veículo (`Vehicle`)

```json
{
  "plate": "ABC-1234",
  "model": "Hatch",
  "color": "Azul",
  "seat_capacity": 4
}
```

### Carona (`Ride`)

```json
{
  "id": "<ride_uuid>",
  "driver_id": "<driver_uuid>",
  "departure_at": "2026-09-17T08:00:00Z",
  "segments": [
    {
      "id": "<segment_uuid>",
      "origin": 3,
      "destination": 1,
      "departure_at": "2026-09-17T08:00:00Z",
      "arrival_at": "2026-09-17T09:30:00Z",
      "price_cents": 2500,
      "available_seats": 3
    }
  ],
  "cancelled": false
}
```

Valores monetários são sempre inteiros em centavos: `2500` representa R$ 25,00.

### Reserva (`Reservation`)

```json
{
  "id": "<reservation_uuid>",
  "passenger_id": "<passenger_uuid>",
  "status": 3,
  "segments": [
    {
      "ride_id": "<ride_uuid>",
      "segment_id": "<segment_uuid>"
    }
  ],
  "created_at": "2026-09-10T15:04:05Z"
}
```

## Ações públicas

### `register_user`

Não requer autenticação. Cria um passageiro ou motorista.

**Requisição**

```json
{
  "action": "register_user",
  "payload": {
    "name": "Ana Souza",
    "email": "ana@example.com",
    "password": "Senha@123",
    "role": "PASSENGER"
  }
}
```

**Sucesso:** `data` contém um [Usuário](#usuário-userresponse). A senha nunca é retornada nem persistida em texto puro.

Validações: nome obrigatório; e-mail válido; senha com no mínimo 8 caracteres, letra, número e caractere especial; e-mail único.

### `login`

Não requer autenticação. Autentica a sessão da conexão atual.

**Requisição**

```json
{
  "action": "login",
  "payload": {
    "email": "ana@example.com",
    "password": "Senha@123"
  }
}
```

**Sucesso:** `data` contém um [Usuário](#usuário-userresponse). A partir desta resposta, as próximas requisições na mesma conexão usam a sessão autenticada.

### `get_my_profile`

Requer autenticação.

**Requisição**

```json
{"action":"get_my_profile","payload":null}
```

**Sucesso:** `data` contém um [Usuário](#usuário-userresponse).

### `logout`

Requer autenticação.

**Requisição**

```json
{"action":"logout","payload":null}
```

**Sucesso:** não possui `data`. A conexão continua aberta, mas volta a ser anônima.

## Ações de motorista

Todas as ações desta seção requerem uma sessão autenticada de usuário com perfil `"DRIVER"`.

### `register_vehicle`

Cria o único veículo associado ao motorista. Falha se ele já tiver veículo.

**Requisição**

```json
{
  "action": "register_vehicle",
  "payload": {
    "plate": "ABC-1234",
    "model": "Hatch",
    "color": "Azul",
    "seat_capacity": 4
  }
}
```

**Sucesso:** `data` contém um [Veículo](#veículo-vehicle).

### `get_my_vehicle`

Requer que o motorista possua veículo.

**Requisição**

```json
{"action":"get_my_vehicle","payload":null}
```

**Sucesso:** `data` contém um [Veículo](#veículo-vehicle).

### `update_vehicle`

Requer que o motorista possua veículo. Substitui os dados do veículo atual.

**Requisição:** igual ao payload de [`register_vehicle`](#register_vehicle).

```json
{
  "action": "update_vehicle",
  "payload": {
    "plate": "DEF-5678",
    "model": "Sedan",
    "color": "Preto",
    "seat_capacity": 5
  }
}
```

**Sucesso:** `data` contém um [Veículo](#veículo-vehicle). A capacidade não pode ser reduzida abaixo da capacidade exigida por qualquer carona ativa do motorista. Alterar placa, modelo, cor ou aumentar a capacidade é permitido.

### `remove_vehicle`

Requer que o motorista possua veículo. A remoção é recusada enquanto o motorista possuir alguma carona ativa; cancele essas caronas antes de remover o veículo.

**Requisição**

```json
{"action":"remove_vehicle","payload":null}
```

**Sucesso:** não possui `data`.

### `create_ride`

Publica uma carona. Requer veículo cadastrado. O servidor cria os IDs da carona e dos trechos.

**Requisição**

```json
{
  "action": "create_ride",
  "payload": {
    "departure_at": "2026-09-17T08:00:00Z",
    "segments": [
      {
        "origin": 3,
        "destination": 1,
        "departure_at": "2026-09-17T08:00:00Z",
        "arrival_at": "2026-09-17T09:30:00Z",
        "price_cents": 2500,
        "available_seats": 4
      },
      {
        "origin": 1,
        "destination": 7,
        "departure_at": "2026-09-17T09:45:00Z",
        "arrival_at": "2026-09-17T11:30:00Z",
        "price_cents": 3000,
        "available_seats": 4
      }
    ]
  }
}
```

**Sucesso:** `data` contém uma [Carona](#carona-ride).

Regras: deve existir pelo menos um trecho; a saída da carona precisa ser a saída do primeiro trecho; os trechos devem ser contínuos e cronológicos; origem e destino de cada trecho devem ser diferentes; assentos devem ser positivos e não podem superar a capacidade do veículo.

### `list_my_rides`

**Requisição**

```json
{"action":"list_my_rides","payload":null}
```

**Sucesso:** `data` é uma lista de [Caronas](#carona-ride), incluindo as canceladas.

### `cancel_ride`

Cancela uma carona do próprio motorista.

**Requisição**

```json
{
  "action": "cancel_ride",
  "payload": {"ride_id":"<ride_uuid>"}
}
```

**Sucesso:** não possui `data`. Reservas confirmadas que usem a carona passam para status `Cancelada`.

### `get_ride_passengers`

Retorna passageiros confirmados por trecho de uma carona pertencente ao motorista autenticado.

**Requisição**

```json
{
  "action": "get_ride_passengers",
  "payload": {"ride_id":"<ride_uuid>"}
}
```

**Sucesso:** `data` segue este formato:

```json
{
  "ride_id": "<ride_uuid>",
  "segments": [
    {
      "segment_id": "<segment_uuid>",
      "origin": 3,
      "destination": 1,
      "departure_at": "2026-09-17T08:00:00Z",
      "arrival_at": "2026-09-17T09:30:00Z",
      "passengers": [
        {
          "uuid": "<passenger_uuid>",
          "name": "Ana Souza",
          "email": "ana@example.com",
          "role": "PASSENGER"
        }
      ]
    }
  ]
}
```

Trechos sem reserva possuem `"passengers": []`. Reservas canceladas não aparecem.

## Ações de passageiro

Todas as ações desta seção requerem uma sessão autenticada. Usuários com perfil `"DRIVER"` também podem executar essas ações, pois motorista mantém as capacidades de passageiro.

### `search_itineraries`

Busca caminhos ativos que ligam origem e destino na data indicada.

**Requisição**

```json
{
  "action": "search_itineraries",
  "payload": {
    "origin": 3,
    "destination": 7,
    "date": "2026-09-17T00:00:00Z"
  }
}
```

**Sucesso:** `data` é uma lista, possivelmente vazia, de itinerários:

```json
[
  {
    "segments": [
      {
        "ride_id": "<ride_uuid>",
        "segment_id": "<segment_uuid>",
        "origin": 3,
        "destination": 1,
        "departure_at": "2026-09-17T08:00:00Z",
        "arrival_at": "2026-09-17T09:30:00Z",
        "price_cents": 2500,
        "available_seats": 4
      }
    ],
    "departure_at": "2026-09-17T08:00:00Z",
    "arrival_at": "2026-09-17T09:30:00Z",
    "total_price_cents": 2500
  }
]
```

O servidor considera somente trechos com assentos disponíveis. A saída do próximo trecho deve ocorrer no mesmo instante ou depois da chegada do trecho anterior.

### `confirm_reservation`

Confirma uma reserva atômica dos trechos de um itinerário retornado por `search_itineraries`.

**Requisição**

```json
{
  "action": "confirm_reservation",
  "payload": {
    "segments": [
      {"ride_id":"<ride_uuid>","segment_id":"<segment_uuid>"},
      {"ride_id":"<other_ride_uuid>","segment_id":"<other_segment_uuid>"}
    ]
  }
}
```

**Sucesso:** `data` contém uma [Reserva](#reserva-reservation), com status `3`.

Os trechos devem estar na ordem do itinerário, sem repetição, formar uma rota contínua e estar disponíveis. Se qualquer trecho não puder ser reservado, nenhum assento é alterado.

### `list_my_reservations`

**Requisição**

```json
{"action":"list_my_reservations","payload":null}
```

**Sucesso:** `data` é uma lista de [Reservas](#reserva-reservation), inclusive canceladas.

### `cancel_reservation`

Cancela uma reserva confirmada que pertença ao passageiro autenticado.

**Requisição**

```json
{
  "action": "cancel_reservation",
  "payload": {"reservation_id":"<reservation_uuid>"}
}
```

**Sucesso:** não possui `data`. Os assentos dos trechos da reserva são devolvidos.

## Erros e autorização

Uma ação desconhecida responde:

```json
{"success":"error","message":"Ação desconhecida"}
```

Em ações protegidas, erros comuns são `"Autenticação necessária."` e `"Apenas motoristas podem executar esta operação."`. Clientes devem tomar `success` como a indicação programática de êxito e tratar `message` como texto para pessoas, não como código de erro estável.
