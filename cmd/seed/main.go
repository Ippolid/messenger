// Command seed наполняет чистую БД воспроизводимыми демонстрационными данными.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Ippolid/messenger/internal/auth"
	"github.com/Ippolid/messenger/internal/storage"
)

const defaultDSN = "postgres://messenger:messenger@localhost:5432/messenger?sslmode=disable"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()
	store, err := storage.New(ctx, envOr("DB_DSN", defaultDSN))
	if err != nil {
		return err
	}
	defer store.Close()

	hash, err := auth.HashPassword("password123")
	if err != nil {
		return err
	}
	if err := store.SeedDemo(ctx, demoUsers(hash), demoChats()); err != nil {
		return err
	}
	fmt.Println("seed: created 4 users, 3 chats and 102 messages")
	return nil
}

func demoUsers(hash string) []storage.SeedUser {
	return []storage.SeedUser{
		{Login: "alice", PasswordHash: hash},
		{Login: "bob", PasswordHash: hash},
		{Login: "carol", PasswordHash: hash},
		{Login: "dave", PasswordHash: hash},
	}
}

func demoChats() []storage.SeedChat {
	now := time.Now().UTC().Add(-72 * time.Hour)
	general := makeMessages([]string{"alice", "bob", "carol", "dave"}, generalTexts(), now)
	team := makeMessages([]string{"alice", "bob", "carol"}, teamTexts(), now.Add(20*time.Minute))
	direct := makeMessages([]string{"alice", "bob"}, directTexts(), now.Add(40*time.Minute))
	return []storage.SeedChat{
		{Type: "group", Title: stringPtr("general"), CreatedBy: "alice", Members: []string{"alice", "bob", "carol", "dave"}, Messages: general},
		{Type: "group", Title: stringPtr("team"), CreatedBy: "alice", Members: []string{"alice", "bob", "carol"}, Messages: team},
		{Type: "direct", CreatedBy: "alice", Members: []string{"alice", "bob"}, Messages: direct},
	}
}

func makeMessages(senders, texts []string, started time.Time) []storage.SeedMessage {
	messages := make([]storage.SeedMessage, 0, len(texts))
	for i, body := range texts {
		messages = append(messages, storage.SeedMessage{
			SenderLogin: senders[i%len(senders)],
			Body:        body,
			CreatedAt:   started.Add(time.Duration(i*47) * time.Minute),
		})
	}
	return messages
}

func generalTexts() []string {
	return []string{
		"Доброе утро! Сегодня обсуждаем план недели.", "Я подготовил заметки по архитектуре сервиса.", "Сообщение с напоминанием: стендап в десять.", "Проверил сборку, все тесты зелёные.", "Кто возьмёт задачу по документации?", "Я могу обновить README после обеда.", "Нужна короткая встреча по поиску сообщений.", "Согласен, русский поиск особенно важно проверить.", "Выложил черновик схемы базы данных.", "Спасибо, посмотрю и оставлю комментарии.", "Не забудьте обновить курсор прочитанных сообщений.", "Вечером можно показать демо преподавателю.", "Нашёл хорошую статью про transactional outbox.", "Давайте добавим ссылку в материалы команды.", "На сегодня критичных блокеров нет.", "Пожалуйста, проверьте уведомления в браузере.", "После релиза устроим небольшую ретроспективу.", "Завтра продолжим работу над мессенджером.", "Вопрос по индексу уже решён.", "Отлично, спасибо всем за помощь.", "Созвон перенесён на пятнадцать минут.", "Новый участник получил доступ к общему чату.", "Сохраняйте осмысленные сообщения для демонстрации поиска.", "Проверка изоляции чатов прошла успешно.", "Финальная версия выглядит аккуратно.", "Добавил примеры API в документацию.", "Пора сделать резервную копию базы.", "Команда готова к следующему этапу.", "Напишите, если понадобится помощь с SQL.", "Хорошего вечера и до завтра.", "Сообщения в general теперь выглядят живыми.", "Итоги дня отправлены в общий канал.", "Завтра утром сверим результаты тестирования.", "Спасибо за внимательную проверку кода.",
	}
}

func teamTexts() []string {
	return []string{
		"Команда, начинаю реализацию поиска.", "Проверьте, что tsvector использует русский словарь.", "Поиск слова сообщения должен находить сообщение.", "GIN-индекс уже создан миграцией.", "Я добавил обработчик HTTP для поиска.", "gRPC метод тоже больше не возвращает Unimplemented.", "Нужен общий rate limit для двух транспортов.", "Пусть лимит будет пять сообщений в секунду.", "Burst десять удобен для коротких серий.", "Проверил ResourceExhausted в gRPC интерсепторе.", "HTTP отвечает 429 с понятной ошибкой.", "Веб-клиент показывает ошибку без падения страницы.", "Осталось оформить seed и SQL-запросы.", "Документация должна выполняться на демо-данных.", "Не забудьте EXPLAIN ANALYZE для keyset пагинации.", "Сегодня закроем финальные критерии приёмки.", "Сделал небольшую правку интерфейса поиска.", "Подсветка совпадений помогает читать результаты.", "Код покрыт базовой проверкой limiter.", "После обеда запускаем полную самопроверку.", "Команда отлично справилась с задачей.", "Финальный отчёт почти готов.", "Пожалуйста, не отправляйте секреты в сообщения.", "Проверяющий сможет войти как alice.", "У bob и carol такой же тестовый пароль.", "Собираем скриншот светлого веб-клиента.", "Все важные решения задокументированы.", "Спасибо за ревью и точные замечания.", "Можно переходить к демонстрации.", "Поиск по слову архитектура работает.", "Нагрузка на чат ограничена корректно.", "Проверка завершена без блокеров.", "Командное сообщение для финального поиска.", "До встречи на защите проекта.",
	}
}

func directTexts() []string {
	return []string{
		"Привет, Bob! Удобно обсудить задачу?", "Привет, Alice, конечно.", "Я посмотрела твой комментарий к поиску.", "Спасибо, там есть важное замечание по изоляции.", "Надо убедиться, что чужой чат не читается.", "Да, сервис сначала проверяет участника чата.", "Отлично. А как прошёл rate limit?", "Быстрая отправка получает 429 после burst.", "Звучит правильно, это легко показать на curl.", "Я подготовлю команды для демонстрации.", "Не забудь про seed с осмысленными текстами.", "Уже добавляю сообщения за несколько дней.", "Хорошо, поиск станет нагляднее.", "README тоже нужно сделать понятным с нуля.", "Я допишу таблицу REST и gRPC методов.", "Спасибо, тогда увидимся на проверке.", "Договорились, удачи с финальной сборкой.", "Финальное личное сообщение для поиска.", "Похоже, всё действительно готово.", "Да, можно отправлять проект.", "На всякий случай ещё раз запусти тесты.", "Сделаю это прямо перед защитой.", "Отличная работа, спасибо!", "До завтра.", "Последнее сообщение в личном чате.", "Теперь в seed ровно достаточно примеров.", "Проверка поиска по слову сообщение пройдёт.", "Надёжность важнее спешки.", "Сохраним спокойствие и чистый код.", "Финальная точка поставлена.", "Увидимся на демонстрации.", "Конец тестовой переписки.", "Ещё одно сообщение для истории.", "Спасибо за совместную работу.",
	}
}

func stringPtr(value string) *string { return &value }

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
