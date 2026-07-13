# messenger

Мессенджер (VK Education, кейс №1)

## Стек
- Go, gRPC + protobuf (buf)
- PostgreSQL 16
- Redis
- TUI на Bubble Tea
- docker compose для окружения

## Требования
Нужны только **Go 1.26+** и **Docker** (с плагином compose). Всё остальное
(buf, protoc-плагины, golangci-lint, migrate) объявлено tool-зависимостями в
`go.mod` и качается автоматически при первом `make proto` / `make lint` —
вручную в PATH ничего ставить не надо.

## Установка и запуск

```bash
git clone git@github.com:Ippolid/messenger.git
cd messenger

make dev         # ОДНА команда: поднять PostgreSQL+Redis, сгенерировать proto, применить миграции
```

`make dev` ждёт, пока БД станет healthy, и только потом накатывает миграции —
гонок нет. То же самое по шагам, если нужно по отдельности:

```bash
make up          # поднять PostgreSQL 16 и Redis 7 и дождаться готовности
make proto       # сгенерировать gRPC-код (при первом запуске скачает buf)
make migrate-up  # применить миграции — создать схему БД
make build       # собрать все бинарники
```

Первый `make proto`/`make lint` скачивает инструменты через `go tool` —
нужен доступ к `proxy.golang.org` (в РФ работает без VPN; при недоступности
можно задать зеркало: `go env -w GOPROXY=https://goproxy.io,direct`).

Проверить, что всё поднялось:
```bash
docker compose -f deploy/docker-compose.yml ps   # оба контейнера healthy
make lint                                          # 0 issues
```

Остановить окружение (данные в volume сохраняются):
```bash
make down
```

## Команды Makefile

| Цель | Действие |
|------|----------|
| `make dev` | всё сразу: up + proto + миграции (главная команда запуска) |
| `make up` / `make down` | поднять / остановить PostgreSQL + Redis |
| `make proto` | сгенерировать gRPC-код из proto в `gen/` |
| `make migrate-up` / `make migrate-down` | применить / откатить миграции |
| `make build` | собрать все бинарники |
| `make lint` | статический анализ (golangci-lint) |
| `make test` | юнит- и интеграционные тесты |
| `make run` / `make tui` / `make seed` | сервер / клиент / демо-данные (в разработке) |

Строку подключения к БД можно переопределить: `make migrate-up DB_DSN=...`.

## Структура
```
api/proto/         # определения gRPC (chat.proto)
gen/               # сгенерированный код (buf generate)
cmd/               # бинарники: chat-service, seed, tui
internal/          # auth, chat, storage, broker, search, audit
migrations/        # SQL-миграции (golang-migrate)
deploy/            # docker-compose.yml
docs/              # SQL-отчёты, EXPLAIN
```
