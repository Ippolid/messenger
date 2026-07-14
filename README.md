# messenger

Учебный мессенджер на Go: gRPC и HTTP/JSON используют одну доменную логику,
PostgreSQL хранит данные, Redis доставляет события в веб-клиент через SSE.

## Быстрый запуск

Нужны Go 1.26+ и Docker Compose. С чистой машины достаточно выполнить:

```bash
docker compose -f deploy/docker-compose.yml up -d
make migrate-up
make seed && make run
```

После запуска откройте http://localhost:8080 и войдите как `alice` с паролем
`password123`. Seed создаёт пользователей `alice`, `bob`, `carol`, `dave`, чаты
`general`, `team` и личную переписку alice–bob, а также 102 сообщения.

> `make seed` намеренно очищает таблицы и заполняет их заново: это обеспечивает
> одинаковые демонстрационные данные при каждом запуске.

![Веб-клиент мессенджера](docs/web-client.png)

## Команды

| Команда | Назначение |
| --- | --- |
| `make up` / `make down` | Запустить / остановить PostgreSQL и Redis |
| `make migrate-up` / `make migrate-down` | Применить / откатить миграции |
| `make seed` | Создать демо-данные |
| `make run` | Запустить gRPC `:50051` и HTTP `:8080` |
| `make build`, `make vet`, `make lint`, `make test` | Проверки проекта |

Для иной БД задайте `DB_DSN`, например:

```bash
DB_DSN='postgres://user:pass@host:5432/db?sslmode=disable' make seed
```

## API

Все методы, кроме регистрации и входа, требуют JWT. В gRPC его передают как
metadata `authorization: Bearer <jwt>`, в HTTP — одноимённым заголовком
(для SSE допускается `?token=`).

| gRPC | REST | Назначение | Авторизация |
| --- | --- | --- | --- |
| `Register` | `POST /api/register` | Регистрация | Нет |
| `Login` | `POST /api/login` | Вход и выдача JWT | Нет |
| `CreateChat` | `POST /api/chats` | Создать личный или групповой чат | Да |
| `GetChats` | `GET /api/chats` | Список чатов и непрочитанные | Да |
| `AddMember` | `POST /api/members` | Добавить участника (admin) | Да |
| `RemoveMember` | — | Удалить участника (admin) | Да |
| `SendMessage` | `POST /api/messages` | Отправить сообщение | Да |
| `GetHistory` | `GET /api/history` | История с keyset-курсором `before_id` | Да |
| `Search` | `GET /api/search?chat_id=&q=` | Полнотекстовый поиск по чату | Да |
| `MarkRead` | `POST /api/read` | Сдвинуть read-курсор | Да |
| `SendTyping` | `POST /api/typing` | Индикатор набора | Да |
| `Subscribe` | `GET /api/events` | Поток событий | Да |
| — | `POST /api/chats/delete` | Удалить чат | Да |

`SendMessage` ограничен для каждого пользователя: 5 сообщений в секунду,
burst 10. HTTP возвращает `429`, gRPC — `ResourceExhausted`.

## Архитектура

- Доменная логика расположена в `internal/auth` и `internal/chat`; SQL — только
  в `internal/storage`.
- Изоляция чатов обеспечивается проверкой членства перед чтением и изменением;
  запросы списка чатов связываются с `chat_members` текущего пользователя.
- История применяет keyset-пагинацию (`id < before_id`), индекс
  `idx_messages_chat_id_id`; реальный план — в [docs/explain.txt](docs/explain.txt).
- Поиск использует сгенерированный `tsvector` с русским словарём и GIN-индекс.
- Отправка сообщения и запись в transactional outbox происходят в одной
  транзакции; relay публикует событие в Redis Streams, fanout доставляет его в SSE.

## Документы и критерии приёмки

- ✅ Изоляция: `internal/chat/messages.go`, `internal/storage/chat.go`
- ✅ Admin-only управление: `internal/chat/service.go`
- ✅ bcrypt вместо plaintext: `internal/auth/password.go`
- ✅ Keyset-пагинация: `internal/storage/messages.go`, `docs/explain.txt`
- ✅ tsvector-поиск: `internal/storage/messages.go`
- ✅ Rate limit → `ResourceExhausted`: `internal/ratelimit`, gRPC interceptor
- ✅ Аудит действий: `internal/storage/audit.go`
- ✅ SQL-критерии: [docs/queries.sql](docs/queries.sql)
- ✅ Docker + миграции + seed: `deploy/docker-compose.yml`, `cmd/seed`
- ✅ golangci-lint: [.golangci.yml](.golangci.yml)
