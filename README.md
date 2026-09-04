# VaiJunto — Caronas Compartilhadas

Aplicação de caronas compartilhadas construída em Go para a disciplina de Concorrência e Conectividade. Há um servidor central TCP e clientes de terminal. A comunicação usa mensagens JSON, uma por linha, sobre uma conexão TCP persistente.

O sistema permite que motoristas publiquem caronas com vários trechos e que passageiros encontrem itinerários, reservem seus trechos e cancelem reservas. O repositório protege as reservas concorrentes para que um assento não seja vendido duas vezes.

## Pré-requisitos

- Go 1.27 ou compatível com o projeto
- Um terminal (PowerShell, CMD, bash etc.)

Confira a instalação com:

```bash
go version
```

## Como iniciar

Todos os comandos abaixo devem ser executados na pasta raiz do projeto.

### 1. Inicie o servidor

Em um terminal, execute:

```bash
go run ./cmd/api-server
```

O servidor escuta na porta `8080` e deve mostrar algo semelhante a:

```text
Servidor iniciado em :8080
```

Mantenha esse terminal aberto enquanto usa os clientes. Os registros do servidor informam conexão, sessão, usuário autenticado, ação solicitada e resultado.

### 2. Inicie um cliente

Abra outro terminal na raiz do projeto. Há três opções:

```bash
# Cliente geral: permite escolher Passageiro ou Motorista no cadastro
go run ./cmd/api-client

# Cliente com cadastro orientado para Motorista
go run ./cmd/api-client-driver

# Cliente com cadastro orientado para Passageiro
go run ./cmd/api-client-passenger
```

Todos se conectam a `localhost:8080`. Se o servidor estiver desligado, o cliente oferece as opções de tentar conectar novamente ou sair.

Você pode abrir vários clientes ao mesmo tempo. Isso é útil para testar uma disputa por assentos com diferentes passageiros.

## Roteiro de uso

### Exemplo: publicar uma carona

1. No cliente geral, escolha `1 - Cadastrar`.
2. Informe nome, e-mail e uma senha válida. Escolha o perfil `Motorista`.
3. Escolha `2 - Login` e entre com o e-mail e a senha cadastrados.
4. Em `Opções de motorista`, entre em `1 - Veículo` e cadastre placa, modelo, cor e capacidade.
5. Volte e entre em `2 - Carona` → `1 - Publicar carona`.
6. Informe a data/hora de partida, a quantidade de trechos e os dados de cada trecho.

Datas e horários de carona usam o formato `dd/mm/aaaa hh:mm`; por exemplo, `17/09/2026 08:00`. Os trechos precisam formar uma rota contínua: o destino de um deve ser a origem do próximo, e os horários devem estar em ordem.

O número de assentos de um trecho não pode ultrapassar a capacidade do veículo.

### Exemplo: buscar e reservar como passageiro

1. Em outro cliente, cadastre e acesse um usuário com o perfil `Passageiro`.
2. Entre em `Opções de passageiro` → `1 - Buscar itinerários`.
3. Escolha origem, destino e a data da viagem no formato `dd/mm/aaaa`.
4. Escolha o número de um itinerário exibido e confirme com `s`.

Uma reserva pode ter mais de um trecho. O servidor só confirma a reserva se todos os trechos ainda tiverem assento; ele nunca confirma apenas parte do itinerário. Para trocar de carona, é exigido intervalo mínimo de 15 minutos.

### Outras operações disponíveis

| Perfil | Operações |
| --- | --- |
| Todos | Cadastro, login, perfil e logout |
| Motorista | CRUD de veículo, publicar/listar/cancelar caronas e consultar passageiros confirmados por trecho |
| Passageiro | Buscar itinerários, confirmar/listar/cancelar reservas |

Quando o motorista cancela uma carona, as reservas confirmadas que dependem dela também são canceladas e os assentos dos trechos são devolvidos. O passageiro continua vendo essa reserva no histórico com status `Cancelada`.

## Validações de cadastro

- E-mail deve ter formato válido.
- Senha deve ter pelo menos 8 caracteres e conter letra, número e caractere especial.
- Senhas não são gravadas em texto puro: o servidor salva apenas o hash BCrypt.

## Persistência local

O projeto não usa banco de dados. O servidor salva os dados em arquivos JSON legíveis na pasta `data/`, criando-a quando for necessário:

```text
data/
├── users.json
├── drivers.json
├── rides.json
└── reservations.json
```

Cada alteração relevante é persistida imediatamente, não apenas quando o servidor é encerrado. Assim, os dados continuam disponíveis após reiniciar o servidor.

## Comunicação TCP e JSON

TCP é o transporte: ele entrega uma sequência de bytes confiável entre cliente e servidor. JSON é o formato escolhido para representar as mensagens nesses bytes. O projeto usa `json.Encoder` e `json.Decoder`, que enviam e leem um objeto JSON por vez na mesma conexão TCP.

Toda requisição possui a estrutura:

```json
{
  "action": "login",
  "payload": {
    "email": "ana@example.com",
    "password": "Senha@123"
  }
}
```

E toda resposta possui esta estrutura:

```json
{
  "success": "success",
  "message": "Login realizado com sucesso.",
  "data": {
    "uuid": "...",
    "name": "Ana",
    "email": "ana@example.com",
    "role": "PASSENGER"
  }
}
```

Algumas ações do protocolo são `register_user`, `login`, `logout`, `register_vehicle`, `create_ride`, `search_itineraries`, `confirm_reservation` e `cancel_reservation`. O cliente de terminal já monta essas mensagens; os exemplos acima servem para documentar o formato.

Uma sessão existe apenas no servidor e está associada a uma conexão TCP. Depois do login, o servidor associa aquele usuário à conexão; outra conexão recebe outra sessão. Ao fechar a conexão ou fazer logout, a sessão deixa de estar autenticada.

## Testes

Execute todos os testes automatizados com:

```bash
go test ./...
```

Além de testes de modelos, persistência e regras de negócio, há um teste de integração TCP que abre 24 clientes independentes. Eles fazem login e disputam simultaneamente o mesmo último assento. O resultado esperado é exatamente uma reserva confirmada, sem venda duplicada.

Para executar apenas esse teste de concorrência TCP:

```bash
go test ./internal/server -run TestTCPConcurrentClientsReserveLastSeat -v
```
