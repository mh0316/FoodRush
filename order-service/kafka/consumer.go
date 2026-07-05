package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type PaymentProcessedEvent struct {
	OrderID   string `json:"order_id"`
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
}

type PaymentFailedEvent struct {
	OrderID string `json:"order_id"`
	Reason  string `json:"reason"`
}

type PaymentConsumer struct {
	reader       *kafka.Reader
	updateStatus func(ctx context.Context, orderID string, status string) error
}

func NewPaymentConsumer(
	broker string,
	topics []string,
	groupID string,
	updateStatus func(ctx context.Context, orderID string, status string) error,
) *PaymentConsumer {
	return &PaymentConsumer{
		updateStatus: updateStatus,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        []string{broker},
			GroupTopics:    topics,
			GroupID:        groupID,
			StartOffset:    kafka.FirstOffset,
			MaxBytes:       10 * 1024 * 1024,
			CommitInterval: time.Second,
			MaxWait:        500 * time.Millisecond,
		}),
	}
}

func (c *PaymentConsumer) Start(ctx context.Context) {
	log.Printf("[orders-service] Kafka consumer started for topics: %v", c.reader.Config().GroupTopics)

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("[orders-service] Kafka payment consumer stopped")
				return
			}
			log.Printf("[orders-service] error reading kafka message: %v", err)
			continue
		}

		var newStatus string
		var orderID string

		switch msg.Topic {
		case "foodrush.payments.processed":
			var event PaymentProcessedEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[orders-service] invalid payment.processed event: %v", err)
				continue
			}
			orderID = event.OrderID
			if event.Status == "APPROVED" {
				newStatus = "PAID"
			} else {
				newStatus = "PAYMENT_DECLINED" // Should not happen in this topic, but good to handle
			}

		case "foodrush.payments.failed":
			var event PaymentFailedEvent
			if err := json.Unmarshal(msg.Value, &event); err != nil {
				log.Printf("[orders-service] invalid payment.failed event: %v", err)
				continue
			}
			orderID = event.OrderID
			newStatus = "PAYMENT_DECLINED"

		default:
			log.Printf("[orders-service] unknown topic: %s", msg.Topic)
			continue
		}

		if orderID == "" {
			log.Printf("[orders-service] order_id is empty in event from topic %s", msg.Topic)
			continue
		}

		if err := c.updateStatus(ctx, orderID, newStatus); err != nil {
			log.Printf("[orders-service] failed to update order status for order %s: %v", orderID, err)
			continue
		}

		log.Printf("[orders-service] order %s status updated to %s", orderID, newStatus)
	}
}

func (c *PaymentConsumer) Close() error {
	return c.reader.Close()
}
