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

	if len(orders) > 0 {
		log.Printf("[orders-service] Relay: found %d orders with unprocessed events", len(orders))
	}

	for _, order := range orders {
		for i := range order.Outbox {
			event := order.Outbox[i]
			if event.Processed {
				continue
			}

			// Determinamos el topic. Si event.EventType ya es un topic válido, lo usamos.
			// Según docker-compose y main.go, el topic es foodrush.orders.created
			topic := event.EventType
			if topic == "" {
				topic = "foodrush.orders.created"
			}

			log.Printf("[orders-service] Relay: attempting to publish event %s of type %s for order %s to topic %s", event.Id, event.EventType, order.Id, topic)

			err := r.producer.PublishGeneric(
				ctx,
				topic,
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
				log.Printf("[orders-service] Relay: failed to mark event %s as processed: %v", event.Id, order.Id, err)
			} else {
				log.Printf("[orders-service] Relay: event %s for order %s published and marked as processed", event.Id, order.Id)
			}
		}
	}
}
