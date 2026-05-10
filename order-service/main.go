package main

import (
	"log"
	"net"
	"os"

	pb "foodrush/orders/proto"
	"foodrush/orders/repository"
	"foodrush/orders/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Initialize MongoDB
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}
	
	repo, err := repository.NewMongoDB(mongoURI, "foodrush", "orders")
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	log.Println("Connected to MongoDB")

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
	orderServer := server.NewOrderServer(repo)
	pb.RegisterOrderServiceServer(s, orderServer)
	
	// Register reflection service on gRPC server to allow grpcurl to work
	reflection.Register(s)

	log.Printf("Orders Service listening on %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
