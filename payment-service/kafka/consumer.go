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

func NewConsumer(broker, topic, groupID string, producer *Producer, repo *repository.PaymentRepository) *Consumer {
	return &Consumer{
		producer: producer,
		repo:     repo,
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:        []string{broker},
			Topic:          topic,
			GroupID:        groupID,
			StartOffset:    kafka.FirstOffset,
			MaxBytes:       10 * 1024 * 1024,
			CommitInterval: time.Second,
			MaxWait:        500 * time.Millisecond,
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

		log.Printf("[payments-service] consumed topic=%s correlation_id=%s order_id=%s total=%.2f",
			msg.Topic, event.CorrelationID, event.OrderID, event.Total)

		// Business Logic: Process Payment
		paymentID := uuid.New().String()
		paymentStatus := "APPROVED"
		failureReason := ""
		if event.Total >= 1000 {
			paymentStatus = "DECLINED"
			failureReason = "Amount exceeds the limit"
		}

		payment := &models.Payment{
			ID:              paymentID,
			OrderID:         event.OrderID,
			UserID:          event.UserID,
			Amount:          int64(event.Total),
			MetodoPagoToken: "saga-simulated-token",
			Status:          paymentStatus,
		}

		// Prepare headers for outbox
		headersMap := map[string]string{"correlation_id": event.CorrelationID}
		headersJSON, _ := json.Marshal(headersMap)

		// Atomically save payment and create outbox event
		if paymentStatus == "APPROVED" {
			processedEvent := PaymentProcessedEvent{
				EventID:       uuid.New().String(),
				CorrelationID: event.CorrelationID,
				EventType:     "foodrush.payments.processed",
				Source:        "payments-service",
				OrderID:       event.OrderID,
				PaymentID:     paymentID,
				Status:        paymentStatus,
				Timestamp:     time.Now().UTC().Format(time.RFC3339),
			}
			payload, _ := json.Marshal(processedEvent)
			outboxEvent := &repository.OutboxEvent{
				ID:        processedEvent.EventID,
				EventType: processedEvent.EventType,
				Payload:   string(payload),
				Headers:   string(headersJSON),
			}
			if err := c.repo.CreateWithOutbox(ctx, payment, outboxEvent); err != nil {
				log.Printf("[payments-service] failed to process payment and outbox: %v", err)
			}
		} else { // Declined
			failedEvent := PaymentFailedEvent{
				EventID:       uuid.New().String(),
				CorrelationID: event.CorrelationID,
				EventType:     "foodrush.payments.failed",
				Source:        "payments-service",
				OrderID:       event.OrderID,
				Reason:        failureReason,
				Timestamp:     time.Now().UTC().Format(time.RFC3339),
			}
			payload, _ := json.Marshal(failedEvent)
			outboxEvent := &repository.OutboxEvent{
				ID:        failedEvent.EventID,
				EventType: failedEvent.EventType,
				Payload:   string(payload),
				Headers:   string(headersJSON),
			}
			if err := c.repo.CreateWithOutbox(ctx, payment, outboxEvent); err != nil {
				log.Printf("[payments-service] failed to process payment and outbox: %v", err)
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
