package main

import (
	"context"
	"database/sql"
	"net"
	"os"
	"math/rand"
	"time"

	"github.com/foodrush/observability"
	"github.com/mh0316/catalog/internal/db"
	"github.com/mh0316/catalog/internal/repository"
	pb "github.com/mh0316/catalog/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedCatalogServiceServer
	repo *repository.CatalogRepository
}

func (s *server) ListComercios(ctx context.Context, req *pb.ListComerciosRequest) (*pb.ListComerciosResponse, error) {
	logger := observability.CtxLogger(ctx)
	logger.Info().Bool("solo_activos", req.SoloActivos).Msg("ListComercios")

	comercios, err := s.repo.ListComercios(ctx, req.SoloActivos)
	if err != nil {
		logger.Error().Err(err).Msg("error ListComercios")
		return nil, err
	}

	return &pb.ListComerciosResponse{Comercios: comercios}, nil
}

func (s *server) GetMenuByComercio(ctx context.Context, req *pb.GetMenuByComercioRequest) (*pb.GetMenuByComercioResponse, error) {
	logger := observability.CtxLogger(ctx)

	// Slow path: dormir 3s para evidenciar cuello de botella en métricas y trazas.
	if req.ComercioId == "slow" {
		logger.Warn().Str("comercio_id", req.ComercioId).Msg("slow query simulated")
		time.Sleep(2 * time.Second)
	}

	logger.Info().Str("comercio_id", req.ComercioId).Msg("GetMenuByComercio")

	productos, err := s.repo.GetMenuByComercio(ctx, req.ComercioId)
	if err != nil {
		logger.Error().Err(err).Str("comercio_id", req.ComercioId).Msg("error GetMenuByComercio")
		return nil, err
	}

	return &pb.GetMenuByComercioResponse{Productos: productos}, nil
}

func (s *server) GetProductDetails(ctx context.Context, req *pb.GetProductDetailsRequest) (*pb.Product, error) {
	logger := observability.CtxLogger(ctx)

	// Slow path: dormir 2s y retornar un producto dummy para evidenciar
	// cuello de botella en trazas y dashboards sin romper validaciones de DB.
	if req.Id == "slow" {
		logger.Warn().Str("product_id", req.Id).Msg("slow query simulated")
		time.Sleep(2 * time.Second)
		return &pb.Product{
			Id:         "slow",
			Nombre:     "Producto lento",
			Precio:     1000,
			ComercioId: "c1",
			Disponible: true,
		}, nil
	}

	logger.Info().Str("product_id", req.Id).Msg("GetProductDetails")

	p, err := s.repo.GetProductDetails(ctx, req.Id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, err
		}
		logger.Error().Err(err).Str("product_id", req.Id).Msg("error GetProductDetails")
		return nil, err
	}

	return p, nil
}

func main() {
	_, shutdown, err := observability.InitTracer("catalog-service")
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("inicializar tracer")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	dbConn, err := openDBWithRetry()
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("no se pudo conectar a la DB")
	}
	defer dbConn.Close()
	observability.Logger().Info().Msg("conexión exitosa a PostgreSQL")

	repo := repository.NewCatalogRepository(dbConn)

	// Servidor de métricas Prometheus en puerto independiente.
	observability.StartMetricsServer(getEnv("METRICS_PORT", ":9000"))

	grpcPort := getEnv("GRPC_PORT", "50051")
	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("error al abrir puerto")
	}

	srv := grpc.NewServer(observability.GRPCServerOptions()...)
	pb.RegisterCatalogServiceServer(srv, &server{repo: repo})

	observability.Logger().Info().Str("port", grpcPort).Msg("Catalog Service escuchando")
	if err := srv.Serve(lis); err != nil {
		observability.Logger().Fatal().Err(err).Msg("error al servir gRPC")
	}
}

func openDBWithRetry() (*sql.DB, error) {
	var dbConn *sql.DB
	var err error
	rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 1; i <= 5; i++ {
		dbConn, err = db.NewPostgresDB()
		if err == nil {
			return dbConn, nil
		}
		observability.Logger().Warn().Int("attempt", i).Err(err).Msg("esperando DB")
		time.Sleep(2*time.Second + time.Duration(rand.Intn(300))*time.Millisecond)
	}
	return nil, err
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
