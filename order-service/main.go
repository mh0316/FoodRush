package main

import (
	"log"
	"net"
	"os"
	"math/rand"
	"time"

	catalogpb "foodrush/orders/catalogpb"
	pb "foodrush/orders/proto"
	"foodrush/orders/repository"
	"foodrush/orders/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	catalogConn, err := grpc.Dial(catalogAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to create catalog client: %v", err)
	}
	defer catalogConn.Close()

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
	orderServer := server.NewOrderServer(repo, catalogpb.NewCatalogServiceClient(catalogConn))
	pb.RegisterOrderServiceServer(s, orderServer)
	
	// Register reflection service on gRPC server to allow grpcurl to work
	reflection.Register(s)

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
