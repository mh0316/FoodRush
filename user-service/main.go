package main

import (
	"context"
	"log"
	"net"

	pb "github.com/jesus-acev/user-service/pb"
	"google.golang.org/grpc"
)

// server implementa la interfaz generada por protoc
type server struct {
	pb.UnimplementedUsersServiceServer
}

// CreateUser: implementación mínima con datos hardcodeados
func (s *server) CreateUser(
	ctx context.Context,
	req *pb.CreateUserRequest,
) (*pb.CreateUserResponse, error) {
	return &pb.CreateUserResponse{
		User: &pb.User{
			Id:           "user-123",
			Nombre:       req.Nombre,
			Correo:       req.Correo,
			PaymentToken: req.PaymentToken,
			Status:       "created",
		},
	}, nil
}

// GetUserProfile: implementación mínima con datos hardcodeados
func (s *server) GetUserProfile(
	ctx context.Context,
	req *pb.GetUserProfileRequest,
) (*pb.GetUserProfileResponse, error) {
	return &pb.GetUserProfileResponse{
		User: &pb.User{
			Id:           req.Id,
			Nombre:       "Juan Pérez",
			Correo:       "juan@example.com",
			PaymentToken: "token-xyz-789",
			Status:       "active",
		},
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterUsersServiceServer(srv, &server{})

	log.Println("servidor gRPC escuchando en :50051")
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
