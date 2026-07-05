package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer              *kafka.Writer
	processedTopic      string
	failedTopic         string
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

type PaymentFailedEvent struct {
	EventID       string `json:"event_id"`
	CorrelationID string `json:"correlation_id"`
	EventType     string `json:"event_type"`
	Source        string `json:"source"`
	OrderID       string `json:"order_id"`
	Reason        string `json:"reason"`
	Timestamp     string `json:"timestamp"`
}

func NewProducer(broker, processedTopic, failedTopic string) *Producer {
	return &Producer{
		processedTopic: processedTopic,
		failedTopic:    failedTopic,
		writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) PublishPaymentProcessed(ctx context.Context, event PaymentProcessedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.publish(ctx, p.processedTopic, event.OrderID, event.CorrelationID, body)
}

func (p *Producer) PublishPaymentFailed(ctx context.Context, event PaymentFailedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.publish(ctx, p.failedTopic, event.OrderID, event.CorrelationID, body)
}

func (p *Producer) publish(ctx context.Context, topic, key, correlationID string, body []byte) error {
	err := p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key),
		Value: body,
		Headers: []kafka.Header{
			{Key: "correlation_id", Value: []byte(correlationID)},
		},
		Time: time.Now(),
	})

	if err != nil {
		log.Printf(
			"[payments-service] error publishing Kafka event correlation_id=%s key=%s error=%v",
			correlationID,
			key,
			err,
		)
		return err
	}

	log.Printf(
		"[payments-service] published topic=%s correlation_id=%s key=%s",
		topic,
		correlationID,
		key,
	)

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
