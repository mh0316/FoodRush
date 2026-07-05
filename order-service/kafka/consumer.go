package kafka

import (
	"context"
	"encoding/json"
	"log"

	"github.com/segmentio/kafka-go"
)

type PaymentProcessedEvent struct {
	EventID       string `json:"event_id"`
	CorrelationID string `json:"correlation_id"`
	EventType     string `json:"event_type"`
	Source        string `json:"source"`
	OrderID       string `json:"order_id"`
	PaymentID     string `json:"payment_id"`
	Status        string `json:"status"`
	Timestamp     string `json:"timestamp"`
}

type PaymentConsumer struct {
	reader        *kafka.Reader
	updateStatus  func(ctx context.Context, orderID string, status string) error
}

func NewPaymentConsumer(
	broker string,
	topic string,
	groupID string,
	updateStatus func(ctx context.Context, orderID string, status string) error,
) *PaymentConsumer {
	return &PaymentConsumer{
		updateStatus: updateStatus,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     []string{broker},
			Topic:       topic,
			GroupID:     groupID,
			StartOffset: kafka.FirstOffset,
		}),
	}
}

func (c *PaymentConsumer) Start(ctx context.Context) {
	log.Println("[orders-service] Kafka consumer started for payment.processed events")

	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("[orders-service] Kafka payment consumer stopped")
				return
			}

			log.Printf("[orders-service] error reading payment.processed event: %v", err)
			continue
		}

		var event PaymentProcessedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("[orders-service] invalid payment.processed event: %v message=%s", err, string(msg.Value))
			continue
		}

		log.Printf(
			"[orders-service] consumed topic=%s correlation_id=%s event_type=%s order_id=%s payment_id=%s status=%s",
			msg.Topic,
			event.CorrelationID,
			event.EventType,
			event.OrderID,
			event.PaymentID,
			event.Status,
		)

		newOrderStatus := "PAID"
		if event.Status != "APPROVED" {
			newOrderStatus = "PAYMENT_DECLINED"
		}

		if c.updateStatus == nil {
			log.Printf("[orders-service] updateStatus function is nil correlation_id=%s order_id=%s", event.CorrelationID, event.OrderID)
			continue
		}

		if err := c.updateStatus(ctx, event.OrderID, newOrderStatus); err != nil {
			log.Printf(
				"[orders-service] failed to update order after payment correlation_id=%s order_id=%s status=%s error=%v",
				event.CorrelationID,
				event.OrderID,
				newOrderStatus,
				err,
			)
			continue
		}

		log.Printf(
			"[orders-service] order status updated after payment correlation_id=%s order_id=%s new_status=%s",
			event.CorrelationID,
			event.OrderID,
			newOrderStatus,
		)
	}
}

func (c *PaymentConsumer) Close() error {
	return c.reader.Close()
}