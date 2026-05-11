package main

import (
	"context"
	"log"
	"net"
	"os"

	"github.com/joho/godotenv"
	pb "github.com/gonzalo-fch/PaymentsService/pb"
	"google.golang.org/grpc"
)

// server implementa el servicio
type server struct {
	pb.UnimplementedPaymentsServiceServer
}

// ProcessPayment (implementación mínima)
func (s *server) ProcessPayment(
	ctx context.Context,
	req *pb.ProcessPaymentRequest,
) (*pb.ProcessPaymentResponse, error) {

	return &pb.ProcessPaymentResponse{
		Id:     "pay-123",
		Status: pb.PaymentStatus_APPROVED,
	}, nil
}

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterPaymentsServiceServer(srv, &server{})

	log.Printf("servidor gRPC escuchando en :%s", port)

	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
