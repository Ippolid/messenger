package broker

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	streamKey = "chat.events"
	groupName = "fanout"
)

// Redis реализует Publisher и Consumer поверх Redis Streams.
type Redis struct {
	client   *redis.Client
	consumer string // имя consumer'а внутри группы
}

func NewRedis(addr, consumer string) *Redis {
	return &Redis{
		client:   redis.NewClient(&redis.Options{Addr: addr}),
		consumer: consumer,
	}
}

// EnsureGroup создаёт consumer-группу (идемпотентно). MKSTREAM создаёт и сам стрим.
func (r *Redis) EnsureGroup(ctx context.Context) error {
	err := r.client.XGroupCreateMkStream(ctx, streamKey, groupName, "0").Err()
	// BUSYGROUP — группа уже есть, это не ошибка.
	if err != nil && !isBusyGroup(err) {
		return fmt.Errorf("create group: %w", err)
	}
	return nil
}

func (r *Redis) Publish(ctx context.Context, payload []byte) error {
	return r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{"data": payload},
	}).Err()
}

// Read блокирующе читает до count новых записей группой (block — таймаут в мс).
func (r *Redis) Read(ctx context.Context, count int, block int) ([]Event, error) {
	res, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    groupName,
		Consumer: r.consumer,
		Streams:  []string{streamKey, ">"},
		Count:    int64(count),
		Block:    time.Duration(block) * time.Millisecond,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil // за время block ничего не пришло
	}
	if err != nil {
		return nil, fmt.Errorf("xreadgroup: %w", err)
	}

	var events []Event
	for _, stream := range res {
		for _, msg := range stream.Messages {
			data, _ := msg.Values["data"].(string)
			events = append(events, Event{ID: msg.ID, Payload: []byte(data)})
		}
	}
	return events, nil
}

func (r *Redis) Ack(ctx context.Context, ids ...string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.client.XAck(ctx, streamKey, groupName, ids...).Err()
}

func (r *Redis) Close() error { return r.client.Close() }

func isBusyGroup(err error) bool {
	return err != nil && len(err.Error()) >= 9 && err.Error()[:9] == "BUSYGROUP"
}
