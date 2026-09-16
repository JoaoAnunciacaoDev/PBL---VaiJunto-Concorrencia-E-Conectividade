# VaiJunto — Caronas Compartilhadas

Aplicação de caronas compartilhadas construída em Go para a disciplina de Concorrência e Conectividade. Há um servidor central TCP e clientes de terminal. A comunicação usa mensagens JSON, uma por linha, sobre uma conexão TCP persistente.

O sistema permite que motoristas publiquem caronas com vários trechos e que passageiros encontrem itinerários, reservem seus trechos e cancelem reservas. O repositório protege as reservas concorrentes para que um assento não seja vendido duas vezes.

O [diagrama de arquitetura](diagrams/ArchitectureDiagram.md) mostra o fluxo entre clientes, conexões TCP, sessões, handlers, repositório concorrente e persistência atômica. O [roteiro de apresentação](PRESENTATION.md) organiza uma demonstração completa em 20 minutos.

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

Abra outro terminal na raiz do projeto:

```bash
# Cliente: permite escolher Passageiro ou Motorista no cadastro
go run ./cmd/api-client
```

Por padrão, o cliente se conecta a `localhost:8080`. Para usar um servidor em outra máquina ou porta, informe o endereço:

```bash
go run ./cmd/api-client -addr 192.168.1.50:8080
```

Se o servidor estiver desligado, o cliente oferece as opções de tentar conectar novamente ou sair.

Você pode abrir vários clientes ao mesmo tempo. Isso é útil para testar uma disputa por assentos com diferentes passageiros.

## Execução com Docker

Crie a imagem e inicie o servidor:

```bash
docker compose -f docker/docker-compose.yaml build server
docker compose -f docker/docker-compose.yaml up -d --wait server
```

O servidor publica a porta TCP `8080` e mantém dados e logs no volume nomeado `vaijunto-storage`. Para criar os dados de demonstração e abrir clientes interativos na mesma máquina:

```bash
docker compose -f docker/docker-compose.yaml run --rm seed
docker compose -f docker/docker-compose.yaml run --rm client
```

O mesmo cliente atende os perfis motorista e passageiro. Abra outros terminais e repita o último comando para usar vários clientes simultaneamente.

O cliente não possui dependência obrigatória do serviço `server`. Na mesma máquina, `server:8080` é usado como padrão e resolvido pelo DNS interno do Docker. Em outro computador, o endereço pode ser informado pela variável `VAIJUNTO_SERVER_ADDR` sem iniciar um servidor local.

### Porta exclusiva no computador servidor

A porta interna do contêiner é sempre `8080`, mas a porta publicada no computador pode ser escolhida com `VAIJUNTO_HOST_PORT`. No PowerShell:

```powershell
$env:VAIJUNTO_HOST_PORT="18042"
docker compose -p vaijunto-joao -f docker/docker-compose.yaml up -d --build --wait server
```

O nome informado em `-p` também torna exclusivos os nomes de rede, contêiner e volume desse projeto. Outros projetos podem usar suas próprias portas e nomes no mesmo computador.

### Contêineres em computadores distintos

No computador do servidor, escolha uma porta, inicie somente o serviço `server` e descubra o endereço IPv4 da máquina na rede local:

```powershell
$env:VAIJUNTO_HOST_PORT="18042"
docker compose -p vaijunto-joao -f docker/docker-compose.yaml up -d --build --wait server
```

Garanta que conexões TCP de entrada para a porta escolhida estejam liberadas no firewall.

No computador cliente, construa a imagem sem iniciar o servidor:

```powershell
docker compose -f docker/docker-compose.yaml build server
```

Em seguida, informe o IP e a porta publicados pelo computador servidor e execute somente o cliente:


```powershell
$env:VAIJUNTO_SERVER_ADDR="192.168.1.50:18042"
docker compose -p vaijunto-cliente -f docker/docker-compose.yaml run --rm client
```

O serviço `client` não possui `depends_on`; portanto, esse comando não cria um servidor no computador cliente. Como alternativa, a mesma imagem pode ser executada sem Compose:

```bash
docker run --rm -it vaijunto:local /app/bin/api-client -addr 192.168.1.50:18042
```

Substitua `192.168.1.50` e `18042` pelo IP e pela porta reais do computador servidor. Não use `localhost`: dentro do contêiner ele aponta para o próprio contêiner cliente.

Para encerrar o servidor sem apagar os dados:

```bash
docker compose -f docker/docker-compose.yaml down
```

### Dados de teste

Com o servidor em execução, o comando abaixo cria duas contas de motorista, duas contas de passageiro, veículos e duas caronas que se conectam em Salvador:

```bash
go run ./cmd/seed
```

Por padrão, as caronas são criadas para o dia seguinte. O comando informa a data ao terminar; para escolher outra, use `-date`:

```bash
go run ./cmd/seed -date 20/09/2026
```

Contas: `ana@gmail.com`, `bruno@gmail.com`, `alice@gmail.com` e `beto@gmail.com`. A senha de todas é `1234568Abc#`. O comando pode ser executado novamente: contas e veículos existentes são mantidos, e não publica nova carona para um motorista que já possua caronas.

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

Uma reserva pode ter mais de um trecho. O servidor só confirma a reserva se todos os trechos ainda tiverem assento; ele nunca confirma apenas parte do itinerário. O trecho seguinte deve sair no mesmo instante ou depois da chegada do anterior.

### Outras operações disponíveis

| Perfil | Operações |
| --- | --- |
| Todos | Cadastro, login, perfil e logout |
| Motorista | Todas as operações de passageiro, além de CRUD de veículo, publicar/listar/cancelar caronas e consultar passageiros confirmados por trecho |
| Passageiro | Buscar itinerários, confirmar/listar/cancelar reservas |

Quando o motorista cancela uma carona, as reservas confirmadas que dependem dela também são canceladas e os assentos dos trechos são devolvidos. O passageiro continua vendo essa reserva no histórico com status `Cancelada`.

Motorista também pode atuar como passageiro: buscar itinerários e fazer ou cancelar reservas de caronas de outros motoristas.

Um veículo não pode ser removido enquanto o motorista possuir caronas ativas. Sua capacidade também não pode ser reduzida abaixo da capacidade exigida por essas caronas.

## Validações de cadastro

- E-mail deve ter formato válido.
- Senha deve ter pelo menos 8 caracteres e conter letra, número e caractere especial.
- Senhas não são gravadas em texto puro: o servidor salva apenas o hash BCrypt.

## Persistência local

O projeto não usa banco de dados. Por padrão, o servidor localiza a raiz do projeto (a pasta que contém `go.mod`) e salva todo o estado em um único JSON na pasta `data/`, criando-a quando for necessário. Ao iniciar, ele informa no log o caminho usado:

```text
data/
└── state.json
```

`state.json` possui versão de formato e reúne usuários, motoristas, caronas e reservas. Cada alteração é serializada em um arquivo temporário, sincronizada com `Sync`, fechada e promovida com um único rename atômico. Assim, a confirmação de uma reserva e a redução dos assentos nunca ficam separadas em arquivos diferentes.

Na primeira execução após uma versão antiga do projeto, se `state.json` ainda não existir, o servidor lê `users.json`, `drivers.json`, `rides.json` e `reservations.json` e cria automaticamente o estado unificado. Depois da migração, `state.json` passa a ser a fonte de verdade.

Para usar conscientemente outra pasta — por exemplo, ao testar dados isolados — informe `-data-dir`:

```bash
go run ./cmd/api-server -data-dir ./outro-diretorio
```

Cada alteração relevante é persistida imediatamente, não apenas quando o servidor é encerrado. Assim, os dados continuam disponíveis após reiniciar o servidor.

## Logs do servidor

Os logs aparecem no terminal e também são acrescentados em `logs/server.log`, ao lado da pasta de dados usada pelo servidor. O arquivo registra inicialização, conexões, ações, sessões e resultados, mas não registra senhas.

## Comunicação TCP e JSON

TCP é o transporte: ele entrega uma sequência de bytes confiável entre cliente e servidor. JSON é o formato escolhido para representar as mensagens nesses bytes. Cada objeto ocupa uma linha, possui limite de 5 MiB e é decodificado de forma estrita. O servidor rejeita campos desconhecidos, conteúdo adicional, mensagens truncadas e requisições sem `action`.

O servidor renova um timeout ocioso de leitura de 5 minutos a cada requisição e limita a escrita de cada resposta a 10 segundos. Um cliente parado ou malformado perde apenas a própria conexão; as demais goroutines continuam atendendo normalmente.

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

O catálogo completo das ações, payloads e respostas está em [PROTOCOL.md](PROTOCOL.md).

## Testes

Execute todos os testes automatizados com:

```bash
go test ./...
```

Além de testes de modelos, persistência e regras de negócio, há testes de integração TCP que:

- abrem 24 clientes independentes disputando o último assento, exigindo exatamente um sucesso e 23 recusas;
- reabrem `state.json` para confirmar que existe somente uma reserva persistida;
- tentam reservar um itinerário de dois trechos cujo último trecho está indisponível e comprovam que nenhum trecho foi alterado;
- calculam mínimo, média, p50, p95, máximo e throughput das confirmações concorrentes;
- enviam JSON truncado, campos desconhecidos, payload inválido, mensagem acima do limite e conexão ociosa;
- simulam falha depois do `Sync` e antes do rename de `state.json`.

Para executar apenas esse teste de concorrência TCP:

```bash
go test ./internal/server -run TestTCP -count=10 -v
```

As métricas são impressas com `t.Logf` e são informativas: não há um limite rígido dependente da velocidade da máquina. O critério funcional permanece invariável em todas as execuções: nunca pode haver mais reservas confirmadas que assentos disponíveis.

Em uma execução local de referência no Windows, com dez repetições de 24 clientes, todas mantiveram 1 sucesso e 23 recusas. A latência média por repetição ficou entre aproximadamente 2 ms e 26 ms e o throughput observado entre 900 e 11,9 mil requisições por segundo; a maior latência ocorreu na primeira execução (aquecimento). Esses números descrevem somente a máquina de teste, incluem a resolução do relógio do ambiente e devem ser medidos novamente no laboratório.
