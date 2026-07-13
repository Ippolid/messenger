package chat

import (
	"sync"

	chatv1 "github.com/Ippolid/messenger/gen/chat/v1"
)

// subscriber — один открытый Subscribe-стрим
type subscriber struct {
	userID int64
	ch     chan *chatv1.ServerEvent
}

// Hub — in-memory реестр подписчиков
// Один пользователь может иметь несколько стримов (несколько клиентов)
// Потокобезопасен: все операции под RWMutex.
type Hub struct {
	mu   sync.RWMutex
	subs map[int64]map[*subscriber]struct{}
}

// NewHub создаёт пустой хаб.
func NewHub() *Hub {
	return &Hub{subs: make(map[int64]map[*subscriber]struct{})}
}

// bufferSize — размер буфера канала подписчика. Небольшой буфер сглаживает
// всплески; при переполнении событие для медленного клиента дропается (typing/новые
// сообщения всё равно подтянутся из истории).
const bufferSize = 64

// Subscribe регистрирует нового подписчика пользователя и возвращает его канал
// и функцию отписки. Отписку нужно вызвать при завершении стрима.
func (h *Hub) Subscribe(userID int64) (<-chan *chatv1.ServerEvent, func()) {
	sub := &subscriber{userID: userID, ch: make(chan *chatv1.ServerEvent, bufferSize)}

	h.mu.Lock()
	if h.subs[userID] == nil {
		h.subs[userID] = make(map[*subscriber]struct{})
	}
	h.subs[userID][sub] = struct{}{}
	h.mu.Unlock()

	unsub := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if set, ok := h.subs[userID]; ok {
			delete(set, sub)
			if len(set) == 0 {
				delete(h.subs, userID)
			}
		}
		close(sub.ch)
	}
	return sub.ch, unsub
}

// Publish доставляет событие всем подписчикам из списка recipients.
// Неблокирующая отправка: если буфер подписчика полон, событие для него дропается,
// чтобы один медленный клиент не тормозил остальных.
func (h *Hub) Publish(recipients []int64, ev *chatv1.ServerEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, uid := range recipients {
		for sub := range h.subs[uid] {
			select {
			case sub.ch <- ev:
			default:
				// буфер полон — пропускаем, не блокируемся
			}
		}
	}
}
