package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"math/rand"
	"time"

	pb "github.com/jesus-acev/user-service/pb"
	"github.com/jesus-acev/user-service/internal/repository"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedUsersServiceServer
	repo *repository.UserRepository
}

func (s *server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	if req.Nombre == "" || req.Correo == "" || req.Password == "" || req.PaymentToken == "" {
		return nil, status.Error(codes.InvalidArgument, "nombre, correo, password y payment_token son obligatorios")
	}

	id, err := s.repo.Create(req)
	if err != nil {
		if repository.IsAlreadyExists(err) {
			return nil, status.Error(codes.AlreadyExists, "correo ya registrado")
		}
		return nil, status.Errorf(codes.Internal, "no se pudo crear usuario: %v", err)
	}

	return &pb.CreateUserResponse{User: &pb.User{Id: id, Nombre: req.Nombre, Correo: req.Correo, PaymentToken: req.PaymentToken, Status: "created"}}, nil
}

func (s *server) GetUserProfile(ctx context.Context, req *pb.GetUserProfileRequest) (*pb.GetUserProfileResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id es obligatorio")
	}

	user, err := s.repo.GetByID(req.Id)
	if repository.IsNotFound(err) {
		return nil, status.Error(codes.NotFound, "usuario no encontrado")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "no se pudo obtener perfil: %v", err)
	}

	user.Status = "active"
	return &pb.GetUserProfileResponse{User: user}, nil
}

func buildDSN() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "user_service")
	password := getEnv("DB_PASSWORD", "user_service_pass")
	name := getEnv("DB_NAME", "user_service_db")
	sslmode := getEnv("DB_SSLMODE", "disable")

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, password, name, sslmode)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func openDBWithRetry() (*sql.DB, error) {
	var db *sql.DB
	var err error
	rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 1; i <= 5; i++ {
		db, err = sql.Open("postgres", buildDSN())
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			return db, nil
		}
		log.Printf("⏳ [%d/5] Esperando a que la DB esté lista...", i)
		time.Sleep(2*time.Second + time.Duration(rand.Intn(300))*time.Millisecond)
	}
	return nil, err
}

func main() {
	db, err := openDBWithRetry()
	if err != nil {
		log.Fatalf("no se pudo conectar a PostgreSQL: %v", err)
	}
	defer db.Close()

	repo := repository.NewUserRepository(db)
	grpcPort := getEnv("GRPC_PORT", "50051")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterUsersServiceServer(srv, &server{repo: repo})

	log.Printf("servidor gRPC escuchando en :%s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
