# messenger

Мессенджер (VK Education, кейс №1)

## Стек
- Go, gRPC + protobuf (buf)
- PostgreSQL 16
- Redis
- TUI на Bubble Tea
- docker compose для окружения

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
