-- Все прикладные SQL-запросы проекта.
-- Параметры $1, $2 и т. д. передаются через pgx; DDL схемы вынесен в migrations/.

-- Пользователи: создать пользователя и вернуть его идентификатор.
INSERT INTO users (login, password_hash)
VALUES ($1, $2)
RETURNING id;

-- Пользователи: получить данные пользователя для входа.
SELECT id, login, password_hash, created_at
FROM users
WHERE login = $1;

-- Пользователи: получить идентификатор по логину при создании чата.
SELECT id
FROM users
WHERE login = $1;

-- Пользователи: получить логин отправителя для realtime-события.
SELECT login
FROM users
WHERE id = $1;

-- Refresh-токены: сохранить только хэш токена и срок действия.
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
VALUES ($1, $2, $3, $4);

-- Чаты: создать чат и вернуть его идентификатор.
INSERT INTO chats (type, title, created_by)
VALUES ($1, $2, $3)
RETURNING id;

-- Чаты: добавить создателя как администратора.
INSERT INTO chat_members (chat_id, user_id, role)
VALUES ($1, $2, 'admin');

-- Чаты: добавить остальных участников, не создавая дубль.
INSERT INTO chat_members (chat_id, user_id, role)
VALUES ($1, $2, 'member')
ON CONFLICT DO NOTHING;

-- Личные чаты: найти существующий чат двух пользователей.
SELECT c.id
FROM chats c
JOIN chat_members m1 ON m1.chat_id = c.id AND m1.user_id = $1
JOIN chat_members m2 ON m2.chat_id = c.id AND m2.user_id = $2
WHERE c.type = 'direct'
LIMIT 1;

-- Чаты: получить тип чата.
SELECT type
FROM chats
WHERE id = $1;

-- Чаты: удалить чат; связанные строки удаляются внешними ключами CASCADE.
DELETE FROM chats
WHERE id = $1;

-- Изоляция: получить роль пользователя только в указанном чате.
SELECT role
FROM chat_members
WHERE chat_id = $1 AND user_id = $2;

-- Список доступных пользователю чатов, их последних сообщений и непрочитанных.
WITH my_chats AS (
    SELECT cm.chat_id, cm.last_read_message_id
    FROM chat_members cm
    WHERE cm.user_id = $1
),
last_msg AS (
    SELECT m.chat_id, m.id, m.body, m.created_at,
           ROW_NUMBER() OVER (PARTITION BY m.chat_id ORDER BY m.id DESC) AS rn
    FROM messages m
    JOIN my_chats mc ON mc.chat_id = m.chat_id
)
SELECT c.id, c.type, c.title,
       COALESCE(lm.id, 0) AS last_message_id,
       lm.body AS last_message_body,
       lm.created_at AS last_message_at,
       (SELECT COUNT(*) FROM messages m2
        WHERE m2.chat_id = c.id AND m2.id > mc.last_read_message_id) AS unread_count,
       (SELECT u.login FROM chat_members cm2
        JOIN users u ON u.id = cm2.user_id
        WHERE cm2.chat_id = c.id AND cm2.user_id <> $1
        LIMIT 1) AS peer_login
FROM my_chats mc
JOIN chats c ON c.id = mc.chat_id
LEFT JOIN last_msg lm ON lm.chat_id = c.id AND lm.rn = 1
ORDER BY last_message_id DESC;

-- Участники: добавить участника с указанной ролью.
INSERT INTO chat_members (chat_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- Участники: удалить пользователя из чата.
DELETE FROM chat_members
WHERE chat_id = $1 AND user_id = $2;

-- Realtime: получить получателей события — всех участников чата.
SELECT user_id
FROM chat_members
WHERE chat_id = $1;

-- Сообщения: в одной транзакции сохранить сообщение и вернуть его данные.
INSERT INTO messages (chat_id, sender_id, body)
VALUES ($1, $2, $3)
RETURNING id, created_at;

-- Transactional outbox: сохранить событие нового сообщения в той же транзакции.
INSERT INTO outbox (message_id, payload)
VALUES ($1, $2);

-- История: keyset-пагинация, без OFFSET; 0 означает первую страницу с конца.
SELECT m.id, m.chat_id, m.sender_id, u.login, m.body, m.created_at
FROM messages m
JOIN users u ON u.id = m.sender_id
WHERE m.chat_id = $1 AND ($2 = 0 OR m.id < $2)
ORDER BY m.id DESC
LIMIT $3;

-- Поиск: полнотекстовый поиск по русскому tsvector внутри одного чата.
SELECT m.id, m.chat_id, m.sender_id, u.login, m.body, m.created_at
FROM messages m
JOIN users u ON u.id = m.sender_id
WHERE m.chat_id = $1
  AND m.body_tsv @@ plainto_tsquery('russian', $2)
ORDER BY m.id DESC
LIMIT $3;

-- Read-курсор: сдвинуть только вперёд, не позволяя уменьшить значение.
UPDATE chat_members
SET last_read_message_id = GREATEST(last_read_message_id, $3)
WHERE chat_id = $1 AND user_id = $2;

-- Outbox relay: выбрать неопубликованные события, используя частичный индекс.
SELECT id, payload
FROM outbox
WHERE published_at IS NULL
ORDER BY id
LIMIT $1;

-- Outbox relay: отметить события опубликованными после успешного Redis XADD.
UPDATE outbox
SET published_at = now()
WHERE id = ANY($1);

-- Outbox: записать событие, не привязанное к одному сообщению (например, message.read).
INSERT INTO outbox (message_id, payload)
VALUES (NULL, $1);

-- Аудит: сохранить действие пользователя и JSONB-детали.
INSERT INTO audit_log (user_id, action, entity, details)
VALUES ($1, $2, $3, $4);

-- Seed: очистить все связанные таблицы и начать идентификаторы заново.
TRUNCATE TABLE users RESTART IDENTITY CASCADE;

-- Seed: создать пользователя и вернуть его идентификатор.
INSERT INTO users (login, password_hash)
VALUES ($1, $2)
RETURNING id;

-- Seed: создать демонстрационный чат и вернуть его идентификатор.
INSERT INTO chats (type, title, created_by)
VALUES ($1, $2, $3)
RETURNING id;

-- Seed: добавить участника демонстрационного чата.
INSERT INTO chat_members (chat_id, user_id, role)
VALUES ($1, $2, $3);

-- Seed: добавить сообщение с явным временем создания.
INSERT INTO messages (chat_id, sender_id, body, created_at)
VALUES ($1, $2, $3, $4);
