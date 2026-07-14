-- Топ чатов по количеству сообщений: JOIN связывает чаты и сообщения, GROUP BY считает их.
SELECT c.id, COALESCE(c.title, 'direct') AS chat_name, COUNT(m.id) AS message_count
FROM chats c
LEFT JOIN messages m ON m.chat_id = c.id
GROUP BY c.id, c.title
ORDER BY message_count DESC, c.id;

-- Непрочитанные сообщения alice: подзапрос получает её read-курсор для каждого чата.
SELECT m.id, m.chat_id, m.body, m.created_at
FROM messages m
WHERE m.id > (
    SELECT cm.last_read_message_id
    FROM chat_members cm
    WHERE cm.chat_id = m.chat_id
      AND cm.user_id = (SELECT id FROM users WHERE login = 'alice')
)
AND EXISTS (
    SELECT 1 FROM chat_members cm
    WHERE cm.chat_id = m.chat_id
      AND cm.user_id = (SELECT id FROM users WHERE login = 'alice')
)
ORDER BY m.chat_id, m.id;

-- Последнее сообщение каждого чата: ROW_NUMBER нумерует сообщения внутри каждого чата.
WITH ranked_messages AS (
    SELECT c.id AS chat_id, COALESCE(c.title, 'direct') AS chat_name,
           m.id AS message_id, m.body, m.created_at,
           ROW_NUMBER() OVER (PARTITION BY c.id ORDER BY m.id DESC) AS row_number
    FROM chats c
    JOIN messages m ON m.chat_id = c.id
)
SELECT chat_id, chat_name, message_id, body, created_at
FROM ranked_messages
WHERE row_number = 1
ORDER BY chat_id;
