package chat

import (
	"context"
	"log"
	"time"

	"github.com/Ippolid/messenger/internal/broker"
	"github.com/Ippolid/messenger/internal/storage"
)

const (
	relayInterval = 50 * time.Millisecond
	relayBatch    = 100
)

// Relay периодически вычитывает неопубликованные записи outbox и публикует их
// в брокер. published_at ставится только после успешного XADD — если Redis
// недоступен, запись остаётся и повторится (at-least-once).
type Relay struct {
	pub   broker.Publisher
	store *storage.Storage
}

func NewRelay(pub broker.Publisher, store *storage.Storage) *Relay {
	return &Relay{pub: pub, store: store}
}

func (r *Relay) Run(ctx context.Context) {
	ticker := time.NewTicker(relayInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.drain(ctx)
		}
	}
}

func (r *Relay) drain(ctx context.Context) {
	rows, err := r.store.Chats.FetchUnpublished(ctx, relayBatch)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("relay fetch: %v", err)
		}
		return
	}
	var published []int64
	for _, row := range rows {
		if err := r.pub.Publish(ctx, row.Payload); err != nil {
			// Redis недоступен — прекращаем батч, published_at не ставим.
			// Эти и последующие записи доедут при следующем тике.
			log.Printf("relay publish: %v", err)
			break
		}
		published = append(published, row.ID)
	}
	if err := r.store.Chats.MarkPublished(ctx, published); err != nil {
		log.Printf("relay mark published: %v", err)
	}
}
