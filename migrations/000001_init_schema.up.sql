-- Полная схема мессенджера одной миграцией: таблицы, ограничения, индексы
BEGIN;

-- Пользователи. Пароль хранится только как bcrypt-хэш.
CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    login         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Чаты: direct -2 участника
CREATE TABLE chats (
    id         BIGSERIAL PRIMARY KEY,
    type       TEXT NOT NULL CHECK (type IN ('direct', 'group')),
    title      TEXT,
    created_by BIGINT NOT NULL REFERENCES users (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Участники чата. last_read_message_id — read-курсор (прочитано = id <= курсора).
CREATE TABLE chat_members (
    chat_id              BIGINT NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    user_id              BIGINT NOT NULL REFERENCES users (id),
    role                 TEXT NOT NULL CHECK (role IN ('admin', 'member', 'reader')),
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    joined_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, user_id)
);

-- Сообщения. body_tsv - столбец для полнотекстового поиска (русский).
CREATE TABLE messages (
    id         BIGSERIAL PRIMARY KEY,
    chat_id    BIGINT NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    sender_id  BIGINT NOT NULL REFERENCES users (id),
    body       TEXT NOT NULL,
    body_tsv   tsvector GENERATED ALWAYS AS (to_tsvector('russian', body)) STORED,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Transactional outbox: пишется в одной транзакции с messages.
-- message_id NULL для эфемерных событий (например, message.read),
-- UNIQUE защищает от дублей по конкретному сообщению.
CREATE TABLE outbox (
    id           BIGSERIAL PRIMARY KEY,
    message_id   BIGINT UNIQUE REFERENCES messages (id) ON DELETE CASCADE,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ
);

-- Refresh-токены: хранится только хэш токена.
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id),
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

-- Журнал аудита действий пользователей.
CREATE TABLE audit_log (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT REFERENCES users (id),
    action     TEXT NOT NULL,
    entity     TEXT NOT NULL,
    details    JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Индексы.
-- Лента чата и пагинация истории (WHERE id < cursor).
CREATE INDEX idx_messages_chat_id_id ON messages (chat_id, id DESC);
-- Полнотекстовый поиск по body_tsv.
CREATE INDEX idx_messages_body_tsv ON messages USING gin (body_tsv);
-- Частичный индекс для outbox-relay: сканируются только неопубликованные строки
CREATE INDEX idx_outbox_unpublished ON outbox (id) WHERE published_at IS NULL;
-- Быстрый поиск чатов пользователя
CREATE INDEX idx_chat_members_user_id ON chat_members (user_id);
-- users(login) UNIQUE уже создаёт нужный индекс через ограничение UNIQUE

COMMIT;
