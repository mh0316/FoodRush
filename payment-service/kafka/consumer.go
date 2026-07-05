package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader   *kafka.Reader
	producer *Producer
}

type OrderCreatedEvent struct {
	EventID       string  `json:"event_id"`
	CorrelationID string `json:"correlation_id"`
	EventType     string `json:"event_type"`
	Source        string `json:"source"`
	OrderID       string `json:"order_id"`
	UserID        string `json:"user_id"`
	ComercioID    string `json:"comercio_id"`
	Total         float64 `json:"total"`
	Status        string  `json:"status"`
	Timestamp     string  `json:"timestamp"`
}

func NewConsumer(broker string, topic string, groupID string, producer *Producer) *Consumer {
	return &Consumer{
		producer: producer,
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

		paymentStatus := "APPROVED"

		paymentEvent := PaymentProcessedEvent{
			EventID:       uuid.New().String(),
			CorrelationID: event.CorrelationID,
			EventType:     "payment.processed",
			Source:        "payments-service",
			OrderID:       event.OrderID,
			PaymentID:     uuid.New().String(),
			Status:        paymentStatus,
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}

		if c.producer != nil {
			if err := c.producer.PublishPaymentProcessed(ctx, paymentEvent); err != nil {
				log.Printf(
					"[payments-service] failed to publish payment.processed correlation_id=%s order_id=%s error=%v",
					event.CorrelationID,
					event.OrderID,
					err,
				)
				continue
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}