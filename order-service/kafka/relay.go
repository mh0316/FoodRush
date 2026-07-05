package kafka

import (
	"context"
	"log"
	"time"

	"foodrush/orders/repository"
)

type OutboxRelay struct {
	repo     repository.OrderStore
	producer *Producer
	interval time.Duration
}

func NewOutboxRelay(repo repository.OrderStore, producer *Producer, interval time.Duration) *OutboxRelay {
	return &OutboxRelay{
		repo:     repo,
		producer: producer,
		interval: interval,
	}
}

func (r *OutboxRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	log.Printf("[orders-service] Outbox Relay started with interval %v", r.interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.processOutbox(ctx)
		}
	}
}

func (r *OutboxRelay) processOutbox(ctx context.Context) {
	orders, err := r.repo.GetUnprocessedEvents(ctx)
	if err != nil {
		log.Printf("[orders-service] Relay: failed to get unprocessed events: %v", err)
		return
	}

	for _, order := range orders {
		for _, event := range order.Outbox {
			if event.Processed {
				continue
			}

			err := r.producer.PublishGeneric(
				ctx,
				event.EventType,
				order.Id,
				[]byte(event.Payload),
				event.Headers,
			)

			if err != nil {
				log.Printf("[orders-service] Relay: failed to publish event %s for order %s: %v", event.Id, order.Id, err)
				continue
			}

			err = r.repo.MarkEventAsProcessed(ctx, order.Id, event.Id)
			if err != nil {
				log.Printf("[orders-service] Relay: failed to mark event %s as processed: %v", event.Id, err)
			} else {
				log.Printf("[orders-service] Relay: event %s for order %s published and marked as processed", event.Id, order.Id)
			}
		}
	}
}
