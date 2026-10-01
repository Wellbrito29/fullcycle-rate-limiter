# fullcycle-rate-limiter

Rate limiter em Go aplicado como middleware HTTP. Limita requisicoes por segundo por endereco IP ou por token de acesso, com persistencia em Redis e bloqueio temporario de quem estourar o limite.

## Como funciona

1. O middleware extrai o IP do cliente e o token do header `API_KEY`.
2. Se existe token, a politica do token e usada; caso contrario vale a politica do IP (token sempre tem precedencia).
3. O contador da chave e incrementado dentro de uma janela de 1 segundo.
4. Se o total passar do limite, a chave entra em bloqueio pelo tempo configurado e todas as novas requisicoes sao recusadas.

Resposta de bloqueio:

```
HTTP/1.1 429 Too Many Requests

you have reached the maximum number of requests or actions allowed within a certain time frame
```

Em requisicoes permitidas o middleware tambem devolve `X-RateLimit-Limit`, `X-RateLimit-Remaining` e `X-RateLimit-Policy` (`ip` ou `token`).

## Como executar

Pre-requisitos: Docker e Docker Compose.

```bash
cp .env.example .env     # opcional, o .env do repositorio ja vem configurado
docker compose up -d --build app
```

A aplicacao sobe na porta `8080` e o Redis na `6379`.

```bash
curl -i http://localhost:8080/
curl -i -H "API_KEY: abc123" http://localhost:8080/
```

Para ver o limite em acao (o `.env` padrao permite 10 req/s por IP):

```bash
./scripts/load.sh 15            # dispara 15 requisicoes sem token
./scripts/load.sh 15 trial      # o token trial permite apenas 3 req/s
```

Derrubar tudo:

```bash
docker compose down -v
```

## Testes

Todos os testes (unitarios + integracao com Redis) rodam dentro do Docker:

```bash
docker compose --profile test run --rm tests
```

Somente os testes unitarios, sem Docker:

```bash
go test ./...
```

Os testes de integracao ficam atras da tag `integration` e dependem de um Redis acessivel:

```bash
REDIS_ADDR=localhost:6379 go test -tags integration ./...
```

Cobertura dos testes:

- `internal/limiter`: limite por IP, isolamento entre chaves, expiracao da janela, duracao do bloqueio, precedencia do token sobre o IP, limite individual por token e concorrencia.
- `internal/middleware`: status `429`, corpo exato da mensagem, leitura do header `API_KEY`, headers de limite e resolucao de IP via `X-Forwarded-For`.
- `internal/config`: valores padrao, leitura das variaveis de ambiente e rejeicao de configuracao invalida.
- `internal/storage/redis`: contagem na janela, expiracao do bloqueio e limiter completo usando Redis.

## Configuracao

Todas as definicoes vem de variaveis de ambiente, lidas do ambiente do processo ou do `.env` na raiz.

| Variavel | Padrao | Descricao |
| --- | --- | --- |
| `SERVER_PORT` | `8080` | Porta HTTP da aplicacao |
| `REDIS_ADDR` | `localhost:6379` | Endereco do Redis |
| `REDIS_PASSWORD` | vazio | Senha do Redis |
| `REDIS_DB` | `0` | Banco do Redis |
| `RATE_LIMIT_IP_MAX_REQUESTS` | `10` | Maximo de requisicoes por segundo por IP |
| `RATE_LIMIT_IP_BLOCK_DURATION` | `5m` | Tempo de bloqueio do IP infrator |
| `RATE_LIMIT_TOKEN_MAX_REQUESTS` | `100` | Maximo de requisicoes por segundo para tokens sem configuracao propria |
| `RATE_LIMIT_TOKEN_BLOCK_DURATION` | `5m` | Tempo de bloqueio do token infrator |
| `RATE_LIMIT_TOKENS` | vazio | Limites individuais por token |

Duracoes aceitam o formato do Go (`5m`, `30s`, `1h`) ou um numero puro, interpretado como segundos.

### Limites por token

`RATE_LIMIT_TOKENS` recebe uma lista separada por virgula no formato `token:limite[:bloqueio]`:

```env
RATE_LIMIT_TOKENS=abc123:100,premium:1000:1m,trial:3:30s
```

No exemplo acima, com `RATE_LIMIT_IP_MAX_REQUESTS=10`, uma requisicao com `API_KEY: abc123` pode fazer 100 req/s, porque a configuracao do token se sobrepoe a do IP. Tokens que nao aparecem na lista usam `RATE_LIMIT_TOKEN_MAX_REQUESTS`.

## Estrutura

```
cmd/server            inicializacao do servidor HTTP
internal/config       leitura da configuracao via ambiente/.env
internal/limiter      regra de negocio do limiter e interface de persistencia
internal/middleware   middleware HTTP que aplica o limiter
internal/storage      implementacoes de persistencia (redis, memory)
```

A regra de negocio vive em `internal/limiter` e nao conhece HTTP; o middleware apenas traduz a requisicao para `limiter.Request` e a decisao para uma resposta HTTP.

## Trocando a estrategia de persistencia

O limiter depende apenas da interface `limiter.Storage`:

```go
type Storage interface {
	Hit(ctx context.Context, key string, window time.Duration) (int64, error)
	Block(ctx context.Context, key string, duration time.Duration) error
	IsBlocked(ctx context.Context, key string) (bool, error)
}
```

Hoje existem duas implementacoes: `internal/storage/redis` (usada pela aplicacao) e `internal/storage/memory` (usada nos testes). Para usar outro mecanismo:

1. Crie um pacote em `internal/storage/<novo>` com um tipo que implemente os tres metodos.
2. Em `cmd/server/main.go`, substitua a construcao do storage:

```go
storage := redisstorage.New(redisstorage.Options{Addr: cfg.RedisAddr})
```

por

```go
storage := novostorage.New(...)
```

Nada mais precisa mudar: `limiter.New` recebe qualquer `Storage` e o middleware permanece igual.
