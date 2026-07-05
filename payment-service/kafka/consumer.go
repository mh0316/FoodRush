package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/gonzalo-fch/PaymentsService/internal/models"
	"github.com/gonzalo-fch/PaymentsService/internal/repository"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader   *kafka.Reader
	producer *Producer
	repo     *repository.PaymentRepository
}

type OrderCreatedEvent struct {
	// ... (no changes here but I'll ensure it matches)
	EventID       string  `json:"event_id"`
	CorrelationID string  `json:"correlation_id"`
	EventType     string  `json:"event_type"`
	Source        string  `json:"source"`
	OrderID       string  `json:"order_id"`
	UserID        string  `json:"user_id"`
	ComercioID    string  `json:"comercio_id"`
	Total         float64 `json:"total"`
	Status        string  `json:"status"`
	Timestamp     string  `json:"timestamp"`
}

func NewConsumer(broker string, topic string, groupID string, producer *Producer, repo *repository.PaymentRepository) *Consumer {
	return &Consumer{
		producer: producer,
		repo:     repo,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     []string{broker},
			Topic:       topic,
			GroupID:     groupID,
			StartOffset: kafka.FirstOffset,
		}),
	}
}

func (c *Consumer) Start(ctx context.Context) {
	log.Println("[payments-service] Kafka consumer started for order.created events")

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("[payments-service] Kafka consumer stopped")
				return
			}

			log.Printf("[payments-service] error reading Kafka message: %v", err)
			continue
		}

		var event OrderCreatedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("[payments-service] invalid order.created event: %v message=%s", err, string(msg.Value))
			continue
		}

		log.Printf(
			"[payments-service] consumed topic=%s correlation_id=%s event_type=%s order_id=%s total=%.2f",
			msg.Topic,
			event.CorrelationID,
			event.EventType,
			event.OrderID,
			event.Total,
		)

		// Business Logic: Process Payment
		paymentID := uuid.New().String()
		paymentStatus := "APPROVED"
		if event.Total > 1000 { // Just an example for failure case
			paymentStatus = "DECLINED"
		}

		payment := &models.Payment{
			ID:              paymentID,
			OrderID:         event.OrderID,
			UserID:          event.UserID,
			Amount:          int64(event.Total),
			MetodoPagoToken: "saga-simulated-token",
			Status:          paymentStatus,
		}

		paymentProcessedEvent := PaymentProcessedEvent{
			EventID:       uuid.New().String(),
			CorrelationID: event.CorrelationID,
			EventType:     "foodrush.payments.processed",
			Source:        "payments-service",
			OrderID:       event.OrderID,
			PaymentID:     paymentID,
			Status:        paymentStatus,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}

		payload, _ := json.Marshal(paymentProcessedEvent)
		headersMap := map[string]string{
			"correlation_id": event.CorrelationID,
		}
		headersJSON, _ := json.Marshal(headersMap)

		outboxEvent := &repository.OutboxEvent{
			ID:        paymentProcessedEvent.EventID,
			EventType: paymentProcessedEvent.EventType,
			Payload:   string(payload),
			Headers:   string(headersJSON),
		}

		// PERSIST PAYMENT AND OUTBOX ATOMICALLY
		if err := c.repo.CreateWithOutbox(ctx, payment, outboxEvent); err != nil {
			log.Printf("[payments-service] failed to process payment and outbox correlation_id=%s order_id=%s error=%v",
				event.CorrelationID, event.OrderID, err)
			continue
		}

		log.Printf("[payments-service] payment processed and outbox event created correlation_id=%s order_id=%s status=%s",
			event.CorrelationID, event.OrderID, paymentStatus)
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
