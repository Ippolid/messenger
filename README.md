# messenger

Учебный мессенджер на Go: gRPC и HTTP/JSON используют одну доменную логику,
PostgreSQL хранит данные, Redis доставляет события в веб-клиент через SSE.

## Отчет и презентация

[Отчет](messenger_report.pdf) и [презентация](messenger_vk_education.pptx) лежат в репозитории

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

## Скрипты и команды

В [Makefile](Makefile) собраны сценарии запуска окружения, миграций, seed и
проверки кода. Скрипт [cmd/seed](cmd/seed/main.go) заполняет БД воспроизводимыми
демонстрационными данными; его запускает команда `make seed`.

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

## Аутентификация и аудит

Все методы, кроме регистрации и входа, требуют JWT. В gRPC его передают как
metadata `authorization: Bearer <jwt>`, в HTTP — одноимённым заголовком
(для SSE допускается `?token=`).

Действия пользователей сохраняются отдельно в таблице `audit_log`: регистрация,
вход, отправка сообщения, отметка прочтения, добавление и удаление участников,
а также удаление чата. В журнал не попадают пароли, хэши и токены.

## Роли и права доступа

| Роль | Права |
| --- | --- |
| `member` | Читать историю и искать сообщения своего чата, отправлять сообщения, отмечать их прочитанными, отправлять typing-события. Может создать новый чат. |
| `reader` | Читать историю и искать сообщения своего чата, отмечать их прочитанными и видеть события, но не отправлять сообщения. |
| `admin` | Все права `member`, а в групповом чате — добавление и удаление участников и удаление чата. Создатель чата становится admin. |

Личный чат может удалить любой его участник; групповой — только admin. Любые
чтение и изменение доступны только участнику соответствующего чата.

## API

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

## SQL

- [docs/all-queries.sql](docs/all-queries.sql) содержит все прикладные SQL-запросы,
  используемые репозиториями, outbox и seed, с поясняющими комментариями.
- В папке [migrations](migrations) лежат SQL-миграции схемы: создание таблиц,
  ограничений, внешних ключей и индексов, а также их откат.
- В [docs/queries.sql](docs/queries.sql) приведены отдельные демонстрационные
  запросы для критериев приёмки.

## ER-диаграмма

Список таблиц и порядок их зависимостей подтверждает
[`000001_init_schema.down.sql`](migrations/000001_init_schema.down.sql); сами
внешние ключи и кратности определены в прямой миграции
[`000001_init_schema.up.sql`](migrations/000001_init_schema.up.sql), поскольку
down-миграция только удаляет таблицы.

```mermaid
erDiagram
    USERS ||--o{ CHATS : "created_by FK"
    USERS ||--o{ CHAT_MEMBERS : "user_id FK"
    CHATS ||--o{ CHAT_MEMBERS : "chat_id FK"
    USERS ||--o{ MESSAGES : "sender_id FK"
    CHATS ||--o{ MESSAGES : "chat_id FK"
    MESSAGES o|--o| OUTBOX : "message_id FK, UNIQUE, nullable"
    USERS ||--o{ REFRESH_TOKENS : "user_id FK"
    USERS o|--o{ AUDIT_LOG : "user_id FK, nullable"

    USERS {
        bigint id PK
        text login UK
        text password_hash
    }
    CHATS {
        bigint id PK
        text type
        bigint created_by FK
    }
    CHAT_MEMBERS {
        bigint chat_id PK, FK
        bigint user_id PK, FK
        text role
        bigint last_read_message_id
    }
    MESSAGES {
        bigint id PK
        bigint chat_id FK
        bigint sender_id FK
        text body
    }
    OUTBOX {
        bigint id PK
        bigint message_id FK, UK
        jsonb payload
    }
    REFRESH_TOKENS {
        uuid id PK
        bigint user_id FK
        text token_hash
    }
    AUDIT_LOG {
        bigint id PK
        bigint user_id FK
        text action
    }
```

`CHAT_MEMBERS` — таблица-связка, поэтому `USERS` и `CHATS` образуют связь N:N:
каждый пользователь может состоять во множестве чатов, а каждый чат содержит
множество пользователей. Остальные связи на диаграмме — 1:N, кроме опциональной
1:1 связи `MESSAGES` ↔ `OUTBOX` (ограничение `UNIQUE` на `outbox.message_id`).

## Архитектура

```mermaid
flowchart LR
    WEB["Веб-клиент\nHTML + REST + SSE"]
    GRPC_CLIENT["gRPC-клиент"]
    HTTP["HTTP transport\ninternal/transport/httpapi"]
    GRPC["gRPC transport\ninternal/transport/grpc"]
    AUTH["Auth\ninternal/auth"]
    CHAT["Chat domain\ninternal/chat"]
    STORAGE["Storage\ninternal/storage"]
    PG[("PostgreSQL\nusers, chats, messages, outbox")]
    RELAY["Outbox relay"]
    REDIS[("Redis Streams")]
    FANOUT["Fanout"]

    WEB -->|REST| HTTP
    WEB <-->|SSE| HTTP
    GRPC_CLIENT --> GRPC
    HTTP --> AUTH
    HTTP --> CHAT
    GRPC --> AUTH
    GRPC --> CHAT
    AUTH --> STORAGE
    CHAT --> STORAGE
    STORAGE --> PG
    PG --> RELAY
    RELAY --> REDIS
    REDIS --> FANOUT
    FANOUT -->|события чатов| HTTP
```

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
