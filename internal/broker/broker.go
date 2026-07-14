// Package broker — интерфейс брокера событий и реализация поверх Redis Streams.
package broker

import "context"

// Event — одно событие в стриме: сырой JSON-payload из outbox и его ack-идентификатор.
type Event struct {
	ID      string // идентификатор записи в стриме (для XACK)
	Payload []byte
}

// Publisher публикует payload в брокер.
type Publisher interface {
	Publish(ctx context.Context, payload []byte) error
}

// Consumer читает события группой и подтверждает обработанные.
type Consumer interface {
	Read(ctx context.Context, count int, block int) ([]Event, error)
	Ack(ctx context.Context, ids ...string) error
}
