package main

import (
	"context"
	"database/sql"
	"net"
	"os"
	"time"

	"github.com/foodrush/observability"
	"github.com/google/uuid"
	pb "github.com/gonzalo-fch/PaymentsService/pb"
	"github.com/gonzalo-fch/PaymentsService/internal/db"
	"github.com/gonzalo-fch/PaymentsService/internal/models"
	"github.com/gonzalo-fch/PaymentsService/internal/repository"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedPaymentsServiceServer
	repo *repository.PaymentRepository
}

func (s *server) ProcessPayment(ctx context.Context, req *pb.ProcessPaymentRequest) (*pb.ProcessPaymentResponse, error) {
	logger := observability.CtxLogger(ctx)

	payment := &models.Payment{
		ID:              uuid.NewString(),
		OrderID:         req.OrderId,
		UserID:          req.UserId,
		Amount:          req.Amount,
		MetodoPagoToken: req.MetodoPagoToken,
		Status:          "APPROVED",
	}

	if err := s.repo.Create(ctx, payment); err != nil {
		logger.Error().Err(err).Msg("error creando pago")
		return nil, err
	}

	logger.Info().Str("payment_id", payment.ID).Str("order_id", payment.OrderID).Msg("pago procesado")
	return &pb.ProcessPaymentResponse{Id: payment.ID, Status: pb.PaymentStatus_APPROVED}, nil
}

func (s *server) GetPaymentByOrder(ctx context.Context, req *pb.GetPaymentByOrderRequest) (*pb.GetPaymentByOrderResponse, error) {
	logger := observability.CtxLogger(ctx)

	payment, err := s.repo.GetByOrderID(ctx, req.OrderId)
	if err != nil {
		logger.Error().Err(err).Str("order_id", req.OrderId).Msg("error obteniendo pago")
		return nil, err
	}

	status := pb.PaymentStatus_PENDING
	switch payment.Status {
	case "APPROVED":
		status = pb.PaymentStatus_APPROVED
	case "DECLINED":
		status = pb.PaymentStatus_DECLINED
	}

	logger.Info().Str("order_id", req.OrderId).Str("payment_id", payment.ID).Msg("pago obtenido")
	return &pb.GetPaymentByOrderResponse{Id: payment.ID, Amount: payment.Amount, Status: status}, nil
}

func main() {
	_ = godotenv.Load()

	_, shutdown, err := observability.InitTracer("payments-service")
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("inicializar tracer")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	database, err := openDBWithRetry()
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("no se pudo conectar a PostgreSQL")
	}
	defer database.Close()

	repo := repository.NewPaymentRepository(database)

	// Servidor de métricas Prometheus en puerto independiente.
	observability.StartMetricsServer(getEnv("METRICS_PORT", ":9000"))

	port := getEnv("PORT", "50051")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("no se pudo escuchar")
	}

	srv := grpc.NewServer(observability.GRPCServerOptions()...)
	pb.RegisterPaymentsServiceServer(srv, &server{repo: repo})

	observability.Logger().Info().Str("port", port).Msg("servidor gRPC escuchando")
	if err := srv.Serve(lis); err != nil {
		observability.Logger().Fatal().Err(err).Msg("error al servir")
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

		observability.Logger().Warn().Int("attempt", i).Err(err).Msg("esperando DB de pagos")
		time.Sleep(2 * time.Second)
	}

	return nil, err
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
