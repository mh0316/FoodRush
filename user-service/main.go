package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"os"
	"math/rand"
	"time"

	"github.com/foodrush/observability"
	pb "github.com/jesus-acev/user-service/pb"
	"github.com/jesus-acev/user-service/internal/repository"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type server struct {
	pb.UnimplementedUsersServiceServer
	repo *repository.UserRepository
}

func (s *server) CreateUser(ctx context.Context, req *pb.CreateUserRequest) (*pb.CreateUserResponse, error) {
	logger := observability.CtxLogger(ctx)

	if req.Nombre == "" || req.Correo == "" || req.Password == "" || req.PaymentToken == "" {
		return nil, status.Error(codes.InvalidArgument, "nombre, correo, password y payment_token son obligatorios")
	}

	id, err := s.repo.Create(ctx, req)
	if err != nil {
		if repository.IsAlreadyExists(err) {
			return nil, status.Error(codes.AlreadyExists, "correo ya registrado")
		}
		logger.Error().Err(err).Msg("error creando usuario")
		return nil, status.Errorf(codes.Internal, "no se pudo crear usuario: %v", err)
	}

	logger.Info().Str("user_id", id).Msg("usuario creado")
	return &pb.CreateUserResponse{User: &pb.User{Id: id, Nombre: req.Nombre, Correo: req.Correo, PaymentToken: req.PaymentToken, Status: "created"}}, nil
}

func (s *server) GetUserProfile(ctx context.Context, req *pb.GetUserProfileRequest) (*pb.GetUserProfileResponse, error) {
	logger := observability.CtxLogger(ctx)

	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id es obligatorio")
	}

	// Error path: simula base de datos caída para evidenciar alertas y trazas de error.
	if req.Id == "error" {
		err := fmt.Errorf("simulacion db caida: conexion rechazada")
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			span.RecordError(err)
		}
		logger.Error().Err(err).Msg("db caída simulada")
		return nil, status.Error(codes.Internal, "db caída simulada")
	}

	user, err := s.repo.GetByID(ctx, req.Id)
	if repository.IsNotFound(err) {
		return nil, status.Error(codes.NotFound, "usuario no encontrado")
	}
	if err != nil {
		logger.Error().Err(err).Str("user_id", req.Id).Msg("error obteniendo perfil")
		return nil, status.Errorf(codes.Internal, "no se pudo obtener perfil: %v", err)
	}

	user.Status = "active"
	logger.Info().Str("user_id", req.Id).Msg("perfil obtenido")
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
		observability.Logger().Warn().Int("attempt", i).Err(err).Msg("esperando DB")
		time.Sleep(2*time.Second + time.Duration(rand.Intn(300))*time.Millisecond)
	}
	return nil, err
}

func main() {
	_, shutdown, err := observability.InitTracer("user-service")
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("inicializar tracer")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	db, err := openDBWithRetry()
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("no se pudo conectar a PostgreSQL")
	}
	defer db.Close()

	repo := repository.NewUserRepository(db)

	// Servidor de métricas Prometheus en puerto independiente.
	observability.StartMetricsServer(getEnv("METRICS_PORT", ":9000"))

	grpcPort := getEnv("GRPC_PORT", "50051")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("no se pudo escuchar")
	}

	srv := grpc.NewServer(observability.GRPCServerOptions()...)
	pb.RegisterUsersServiceServer(srv, &server{repo: repo})

	observability.Logger().Info().Str("port", grpcPort).Msg("servidor gRPC escuchando")
	if err := srv.Serve(lis); err != nil {
		observability.Logger().Fatal().Err(err).Msg("error al servir")
	}
}
