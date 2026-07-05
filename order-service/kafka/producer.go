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

type OrderCreatedEvent struct {
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

func NewProducer(broker string, topic string) *Producer {
	return &Producer{
		topic: topic,
		writer: &kafka.Writer{
			Addr:     kafka.TCP(broker),
			Balancer: &kafka.LeastBytes{},
		},
	}
}

func (p *Producer) PublishOrderCreated(ctx context.Context, event OrderCreatedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Topic: p.topic,
		Key:   []byte(event.OrderID),
		Value: body,
		Headers: []kafka.Header{
			{Key: "correlation_id", Value: []byte(event.CorrelationID)},
		},
		Time: time.Now(),
	})

	if err != nil {
		log.Printf("[orders-service] error publishing Kafka event correlation_id=%s order_id=%s error=%v",
			event.CorrelationID, event.OrderID, err)
		return err
	}

	log.Printf("[orders-service] published topic=%s correlation_id=%s event_type=%s order_id=%s",
		p.topic, event.CorrelationID, event.EventType, event.OrderID)

	return nil
}

func (p *Producer) PublishGeneric(ctx context.Context, topic string, key string, payload []byte, headers map[string]string) error {
	var kHeaders []kafka.Header
	for k, v := range headers {
		kHeaders = append(kHeaders, kafka.Header{Key: k, Value: []byte(v)})
	}

	// Creamos un nuevo writer o usamos uno que no tenga el topic fijo si p.writer ya lo tiene.
	// El problema es que si p.writer se creó con un Topic fijo, ignorará el de la p.writer.WriteMessages
	// si no se tiene cuidado con la versión de la librería.
	// Para mayor seguridad en kafka-go, creamos un writer efímero para el topic específico o usamos p.writer si se permite.

	err := p.writer.WriteMessages(ctx, kafka.Message{
		Topic:   topic, // Especificamos el topic aquí
		Key:     []byte(key),
		Value:   payload,
		Headers: kHeaders,
		Time:    time.Now(),
	})

	if err != nil {
		log.Printf("[kafka-producer] error publishing to topic=%s key=%s error=%v", topic, key, err)
		return err
	}

	log.Printf("[kafka-producer] published to topic=%s key=%s", topic, key)
	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
