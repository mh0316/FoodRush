package main

import (
	"context"
	"net"
	"os"
	"math/rand"
	"time"

	"github.com/foodrush/observability"
	catalogpb "foodrush/orders/catalogpb"
	pb "foodrush/orders/proto"
	"foodrush/orders/repository"
	"foodrush/orders/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	_, shutdown, err := observability.InitTracer("orders-service")
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("inicializar tracer")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	repo, err := connectMongoWithRetry(mongoURI)
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("failed to connect to MongoDB")
	}
	observability.Logger().Info().Msg("Connected to MongoDB")

	catalogAddr := os.Getenv("CATALOG_SERVICE_ADDR")
	if catalogAddr == "" {
		catalogAddr = "catalog-service:50051"
	}
	catalogConn, err := grpc.Dial(
		catalogAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		observability.GRPCClientStatsHandler(),
	)
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("failed to create catalog client")
	}
	defer catalogConn.Close()

	// Servidor de métricas Prometheus en puerto independiente.
	observability.StartMetricsServer(getEnv("METRICS_PORT", ":9000"))

	port := getEnv("PORT", "50051")
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		observability.Logger().Fatal().Err(err).Msg("failed to listen")
	}

	s := grpc.NewServer(observability.GRPCServerOptions()...)
	orderServer := server.NewOrderServer(repo, catalogpb.NewCatalogServiceClient(catalogConn))
	pb.RegisterOrderServiceServer(s, orderServer)

	// Register reflection service on gRPC server to allow grpcurl to work
	reflection.Register(s)

	observability.Logger().Info().Str("port", port).Msg("Orders Service listening")
	if err := s.Serve(lis); err != nil {
		observability.Logger().Fatal().Err(err).Msg("failed to serve")
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
		observability.Logger().Warn().Int("attempt", i).Err(err).Msg("waiting for MongoDB")
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
