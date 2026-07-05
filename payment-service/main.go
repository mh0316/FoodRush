package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"time"

	"github.com/gonzalo-fch/PaymentsService/internal/db"
	"github.com/gonzalo-fch/PaymentsService/internal/models"
	"github.com/gonzalo-fch/PaymentsService/internal/repository"
	paymentkafka "github.com/gonzalo-fch/PaymentsService/kafka"
	pb "github.com/gonzalo-fch/PaymentsService/pb"
	"github.com/google/uuid"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedPaymentsServiceServer
	repo *repository.PaymentRepository
}

func (s *server) ProcessPayment(ctx context.Context, req *pb.ProcessPaymentRequest) (*pb.ProcessPaymentResponse, error) {
	payment := &models.Payment{
		ID:              uuid.NewString(),
		OrderID:         req.OrderId,
		UserID:          req.UserId,
		Amount:          req.Amount,
		MetodoPagoToken: req.MetodoPagoToken,
		Status:          "APPROVED",
	}

	if err := s.repo.Create(payment); err != nil {
		return nil, err
	}

	return &pb.ProcessPaymentResponse{
		Id:     payment.ID,
		Status: pb.PaymentStatus_APPROVED,
	}, nil
}

func (s *server) GetPaymentByOrder(ctx context.Context, req *pb.GetPaymentByOrderRequest) (*pb.GetPaymentByOrderResponse, error) {
	payment, err := s.repo.GetByOrderID(req.OrderId)
	if err != nil {
		return nil, err
	}

	status := pb.PaymentStatus_PENDING
	switch payment.Status {
	case "APPROVED":
		status = pb.PaymentStatus_APPROVED
	case "DECLINED":
		status = pb.PaymentStatus_DECLINED
	}

	return &pb.GetPaymentByOrderResponse{
		Id:     payment.ID,
		Amount: payment.Amount,
		Status: status,
	}, nil
}

func main() {
	_ = godotenv.Load()

	database, err := openDBWithRetry()
	if err != nil {
		log.Fatalf("no se pudo conectar a PostgreSQL: %v", err)
	}
	defer database.Close()

	repo := repository.NewPaymentRepository(database)

	// Kafka config
	kafkaBroker := os.Getenv("KAFKA_BROKERS")
	if kafkaBroker == "" {
		kafkaBroker = "kafka:9092"
	}

	orderCreatedTopic := os.Getenv("KAFKA_TOPIC_ORDER_CREATED")
	if orderCreatedTopic == "" {
		orderCreatedTopic = "foodrush.order.created"
	}

	paymentProcessedTopic := os.Getenv("KAFKA_TOPIC_PAYMENT_PROCESSED")
	if paymentProcessedTopic == "" {
		paymentProcessedTopic = "foodrush.payment.processed"
	}

	consumerGroup := os.Getenv("KAFKA_CONSUMER_GROUP")
	if consumerGroup == "" {
		consumerGroup = "payments-service"
	}

	paymentProducer := paymentkafka.NewProducer(kafkaBroker, paymentProcessedTopic)
	defer paymentProducer.Close()

	paymentConsumer := paymentkafka.NewConsumer(
		kafkaBroker,
		orderCreatedTopic,
		consumerGroup,
		paymentProducer,
		repo,
	)
	defer paymentConsumer.Close()

	go paymentConsumer.Start(context.Background())

	// Start Outbox Relay
	relayInterval := 5 * time.Second
	relay := paymentkafka.NewOutboxRelay(repo, paymentProducer, relayInterval)
	go relay.Start(context.Background())

	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterPaymentsServiceServer(srv, &server{repo: repo})

	log.Printf("servidor gRPC escuchando en :%s", port)
	log.Printf("[payments-service] Kafka conectado broker=%s consume_topic=%s produce_topic=%s group=%s",
		kafkaBroker,
		orderCreatedTopic,
		paymentProcessedTopic,
		consumerGroup,
	)

	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}

func openDBWithRetry() (*sql.DB, error) {
	var database *sql.DB
	var err error

	for i := 1; i <= 5; i++ {
		database, err = db.NewPostgresDB()
		if err == nil {
			return database, nil
		}

		log.Printf("⏳ [%d/5] Esperando a que la DB de pagos esté lista...", i)
		time.Sleep(2 * time.Second)
	}

	return nil, err
}
