package main

import (
	"context"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	catalogpb "foodrush/orders/catalogpb"
	orderkafka "foodrush/orders/kafka"
	pb "foodrush/orders/proto"
	"foodrush/orders/repository"
	"foodrush/orders/server"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	repo, err := connectMongoWithRetry(mongoURI)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB")

	catalogAddr := os.Getenv("CATALOG_SERVICE_ADDR")
	if catalogAddr == "" {
		catalogAddr = "catalog-service:50051"
	}

	catalogConn, err := grpc.Dial(
		catalogAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to create catalog client: %v", err)
	}
	defer catalogConn.Close()

	// Kafka broker
	kafkaBroker := os.Getenv("KAFKA_BROKERS")
	if kafkaBroker == "" {
		kafkaBroker = "kafka:9092"
	}

	orderCreatedTopic := os.Getenv("KAFKA_TOPIC_ORDER_CREATED")
	if orderCreatedTopic == "" {
		orderCreatedTopic = "foodrush.orders.created"
	}

	paymentProcessedTopic := os.Getenv("KAFKA_TOPIC_PAYMENT_PROCESSED")
	if paymentProcessedTopic == "" {
		paymentProcessedTopic = "foodrush.payments.processed"
	}

	paymentFailedTopic := os.Getenv("KAFKA_TOPIC_PAYMENT_FAILED")
	if paymentFailedTopic == "" {
		paymentFailedTopic = "foodrush.payments.failed"
	}

	producer := orderkafka.NewProducer(kafkaBroker, orderCreatedTopic)
	defer producer.Close()

	paymentConsumerGroup := os.Getenv("KAFKA_CONSUMER_GROUP")
	if paymentConsumerGroup == "" {
		paymentConsumerGroup = "orders-service-payments-consumer"
	}

	// Wrapper function to match the expected signature
	updateStatusFunc := func(ctx context.Context, orderID string, status string) error {
		return repo.UpdateOrderStatusByID(ctx, orderID, status)
	}

	paymentConsumer := orderkafka.NewPaymentConsumer(
		kafkaBroker,
		[]string{paymentProcessedTopic, paymentFailedTopic},
		paymentConsumerGroup,
		updateStatusFunc,
	)
	defer paymentConsumer.Close()

	go paymentConsumer.Start(context.Background())

	relay := orderkafka.NewOutboxRelay(repo, producer, 5*time.Second)
	go relay.Start(context.Background())

	log.Printf(
		"[orders-service] Kafka conectado broker=%s produce_topic=%s",
		kafkaBroker,
		orderCreatedTopic,
	)

	// Start gRPC server
	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	orderServer := server.NewOrderServer(
		repo,
		catalogpb.NewCatalogServiceClient(catalogConn),
		producer,
	)

	pb.RegisterOrderServiceServer(s, orderServer)
	reflection.Register(s)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	log.Printf("Orders Service listening on %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func connectMongoWithRetry(uri string) (*repository.MongoDB, error) {
	var repo *repository.MongoDB
	var err error

	rand.New(rand.NewSource(time.Now().UnixNano()))

	for i := 1; i <= 5; i++ {
		repo, err = repository.NewMongoDB(uri, "foodrush", "orders")
		if err == nil {
			return repo, nil
		}

		log.Printf("waiting for MongoDB (%d/5): %v", i, err)
		time.Sleep(2*time.Second + time.Duration(rand.Intn(300))*time.Millisecond)
	}

	return nil, err
}
