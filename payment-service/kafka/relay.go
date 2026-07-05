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

	log.Printf("[payment-service] Outbox Relay started with interval %v", r.interval)

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
	log.Println("[payment-service] Relay: Checking for unprocessed events...") // Log de diagnóstico

	events, err := r.repo.GetUnprocessedEvents(ctx)
	if err != nil {
		log.Printf("[payment-service] Relay: failed to get unprocessed events: %v", err)
		return
	}

	if len(events) > 0 {
		log.Printf("[payment-service] Relay: Found %d unprocessed events", len(events))
	}

	for _, event := range events {
		log.Printf("[payment-service] Relay: processing event %s of type %s", event.ID, event.EventType)

		var err error
		switch event.EventType {
		case "foodrush.payments.processed":
			var processedEvent PaymentProcessedEvent
			if err = json.Unmarshal([]byte(event.Payload), &processedEvent); err == nil {
				err = r.producer.PublishPaymentProcessed(ctx, processedEvent)
			}
		case "foodrush.payments.failed":
			var failedEvent PaymentFailedEvent
			if err = json.Unmarshal([]byte(event.Payload), &failedEvent); err == nil {
				err = r.producer.PublishPaymentFailed(ctx, failedEvent)
			}
		default:
			log.Printf("[payment-service] Relay: unknown event type %s for event %s", event.EventType, event.ID)
			continue
		}

		if err != nil {
			log.Printf("[payment-service] Relay: failed to publish event %s: %v", event.ID, err)
			continue
		}

		err = r.repo.MarkEventAsProcessed(ctx, event.ID)
		if err != nil {
			log.Printf("[payment-service] Relay: failed to mark event %s as processed: %v", event.ID, err)
		} else {
			log.Printf("[payment-service] Relay: event %s published and marked as processed", event.ID)
		}
	}
}
