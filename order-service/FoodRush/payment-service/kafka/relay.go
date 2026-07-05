package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gonzalo-fch/PaymentsService/internal/repository"
)

type OutboxRelay struct {
	repo     *repository.PaymentRepository
	producer *Producer
	interval time.Duration
}

func NewOutboxRelay(repo *repository.PaymentRepository, producer *Producer, interval time.Duration) *OutboxRelay {
	return &OutboxRelay{
		repo:     repo,
		producer: producer,
		interval: interval,
	}
}

func (r *OutboxRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	log.Printf("[payments-service] Outbox Relay started with interval %v", r.interval)

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
	events, err := r.repo.GetUnprocessedEvents(ctx)
	if err != nil {
		log.Printf("[payments-service] Relay: failed to get unprocessed events: %v", err)
		return
	}

	for _, event := range events {
		var headers map[string]string
		if err := json.Unmarshal([]byte(event.Headers), &headers); err != nil {
			log.Printf("[payments-service] Relay: failed to unmarshal headers for event %s: %v", event.ID, err)
			headers = make(map[string]string)
		}

		// Use order_id as key if possible from payload
		var payloadMap map[string]interface{}
		json.Unmarshal([]byte(event.Payload), &payloadMap)
		key := event.ID
		if orderID, ok := payloadMap["order_id"].(string); ok {
			key = orderID
		}

		err := r.producer.PublishGeneric(
			ctx,
			r.producer.topic,
			key,
			[]byte(event.Payload),
			headers,
		)

		if err != nil {
			log.Printf("[payments-service] Relay: failed to publish event %s: %v", event.ID, err)
			continue
		}

		err = r.repo.MarkEventAsProcessed(ctx, event.ID)
		if err != nil {
			log.Printf("[payments-service] Relay: failed to mark event %s as processed: %v", event.ID, err)
		} else {
			log.Printf("[payments-service] Relay: event %s published and marked as processed", event.ID)
		}
	}
}
