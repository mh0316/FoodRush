package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/jesus-acev/user-service/pb"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedUsersServiceServer
	db *sql.DB
}

func (s *server) CreateUser(
	ctx context.Context,
	req *pb.CreateUserRequest,
) (*pb.CreateUserResponse, error) {
	if req.Nombre == "" || req.Correo == "" || req.Password == "" || req.PaymentToken == "" {
		return nil, status.Error(codes.InvalidArgument, "nombre, correo, password y payment_token son obligatorios")
	}

	const query = `
		INSERT INTO users (nombre, correo, payment_token, password)
		VALUES ($1, $2, $3, $4)
		RETURNING id;
	`

	var id string
	err := s.db.QueryRowContext(
		ctx,
		query,
		req.Nombre,
		req.Correo,
		req.PaymentToken,
		req.Password,
	).Scan(&id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "no se pudo crear usuario: %v", err)
	}

	return &pb.CreateUserResponse{
		User: &pb.User{
			Id:           id,
			Nombre:       req.Nombre,
			Correo:       req.Correo,
			PaymentToken: req.PaymentToken,
			Status:       "created",
		},
	}, nil
}

func (s *server) GetUserProfile(
	ctx context.Context,
	req *pb.GetUserProfileRequest,
) (*pb.GetUserProfileResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id es obligatorio")
	}

	const query = `
		SELECT id, nombre, correo, payment_token
		FROM users
		WHERE id = $1;
	`

	user := &pb.User{}
	err := s.db.QueryRowContext(ctx, query, req.Id).Scan(
		&user.Id,
		&user.Nombre,
		&user.Correo,
		&user.PaymentToken,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "usuario no encontrado")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "no se pudo obtener perfil: %v", err)
	}

	user.Status = "active"

	return &pb.GetUserProfileResponse{
		User: user,
	}, nil
}

func buildDSN() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "user_service")
	password := getEnv("DB_PASSWORD", "user_service_pass")
	name := getEnv("DB_NAME", "user_service_db")
	sslmode := getEnv("DB_SSLMODE", "disable")

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host,
		port,
		user,
		password,
		name,
		sslmode,
	)
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func main() {
	db, err := sql.Open("postgres", buildDSN())
	if err != nil {
		log.Fatalf("no se pudo crear conexion DB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("no se pudo conectar a PostgreSQL: %v", err)
	}

	grpcPort := getEnv("GRPC_PORT", "50051")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("no se pudo escuchar: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterUsersServiceServer(srv, &server{db: db})

	log.Printf("servidor gRPC escuchando en :%s", grpcPort)
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("error al servir: %v", err)
	}
}
