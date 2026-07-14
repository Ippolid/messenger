package chat

import (
	"context"
	"log"

	"github.com/Ippolid/messenger/internal/broker"
	"github.com/Ippolid/messenger/internal/storage"
)

// Fanout читает события из брокера, восстанавливает ServerEvent и доставляет их
// участникам чата через hub. XACK — только после доставки в hub.
type Fanout struct {
	consumer broker.Consumer
	store    *storage.Storage
	hub      *Hub
}

func NewFanout(consumer broker.Consumer, store *storage.Storage, hub *Hub) *Fanout {
	return &Fanout{consumer: consumer, store: store, hub: hub}
}

// Run блокируется до отмены ctx, читая брокер в цикле.
func (f *Fanout) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		events, err := f.consumer.Read(ctx, 100, 1000) // блок до 1с
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("fanout read: %v", err)
			continue
		}
		var acked []string
		for _, ev := range events {
			f.deliver(ctx, ev.Payload)
			acked = append(acked, ev.ID)
		}
		if err := f.consumer.Ack(ctx, acked...); err != nil {
			log.Printf("fanout ack: %v", err)
		}
	}
}

func (f *Fanout) deliver(ctx context.Context, payload []byte) {
	chatID, err := chatIDFromPayload(payload)
	if err != nil {
		return
	}
	ev, err := serverEventFromPayload(payload)
	if err != nil || ev == nil {
		return
	}
	ids, err := f.store.Chats.MemberIDs(ctx, chatID)
	if err != nil {
		return
	}
	f.hub.Publish(ids, ev)
}
