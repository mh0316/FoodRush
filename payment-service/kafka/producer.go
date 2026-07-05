package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
	topic  string
}

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

func NewProducer(broker string, topic string) *Producer {
	return &Producer{
		topic: topic,
		writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Topic:    topic,
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) PublishPaymentProcessed(ctx context.Context, event PaymentProcessedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.OrderID),
		Value: body,
		Time:  time.Now(),
	})

	if err != nil {
		log.Printf(
			"[payments-service] error publishing Kafka event correlation_id=%s order_id=%s error=%v",
			event.CorrelationID,
			event.OrderID,
			err,
		)
		return err
	}

	log.Printf(
		"[payments-service] published topic=%s correlation_id=%s event_type=%s order_id=%s status=%s",
		p.topic,
		event.CorrelationID,
		event.EventType,
		event.OrderID,
		event.Status,
	)

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}