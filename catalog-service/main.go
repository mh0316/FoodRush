package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"math/rand"
	"time"

	"github.com/mh0316/catalog/internal/db"
	"github.com/mh0316/catalog/internal/repository"
	pb "github.com/mh0316/catalog/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Estructura del servidor con la conexión a la BD
type server struct {
	pb.UnimplementedCatalogServiceServer
	repo *repository.CatalogRepository
}

// Este método consulta la tabla 'comercios'
func (s *server) ListComercios(ctx context.Context, req *pb.ListComerciosRequest) (*pb.ListComerciosResponse, error) {
	log.Printf("📥 ListComercios: SoloActivos=%v", req.SoloActivos)

	comercios, err := s.repo.ListComercios(ctx, req.SoloActivos)
	if err != nil {
		log.Printf("❌ Error en ListComercios: %v", err)
		return nil, err
	}

	return &pb.ListComerciosResponse{Comercios: comercios}, nil
}

// Este método consulta la tabla 'productos' por ID de comercio
func (s *server) GetMenuByComercio(ctx context.Context, req *pb.GetMenuByComercioRequest) (*pb.GetMenuByComercioResponse, error) {
	log.Printf("📥 GetMenuByComercio: ID=%s", req.ComercioId)

	productos, err := s.repo.GetMenuByComercio(ctx, req.ComercioId)
	if err != nil {
		log.Printf("❌ Error en GetMenuByComercio: %v", err)
		return nil, err
	}

	return &pb.GetMenuByComercioResponse{Productos: productos}, nil
}

// Este método consulta la tabla 'productos' por ID de producto
func (s *server) GetProductDetails(ctx context.Context, req *pb.GetProductDetailsRequest) (*pb.Product, error) {
	log.Printf("📥 GetProductDetails: ID=%s", req.Id)

	p, err := s.repo.GetProductDetails(ctx, req.Id)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, status.Error(codes.NotFound, "producto no encontrado")
		}
		log.Printf("❌ Error en GetProductDetails: %v", err)
		return nil, err
	}

	return p, nil
}

// Esta función main inicializa la conexión a la BD y el servidor gRPC.
// Agrega una lógica de reintento para asegurar que el servicio se inicie solo cuando la BD esté lista.
func main() {
	dbConn, err := openDBWithRetry()
	if err != nil {
		log.Fatalf("❌ No se pudo conectar a la DB tras varios intentos: %v", err)
	}
	defer dbConn.Close()
	log.Println("✅ Conexión exitosa a PostgreSQL")
	repo := repository.NewCatalogRepository(dbConn)

	// Se inicia el servidor gRPC
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Error al abrir puerto: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterCatalogServiceServer(srv, &server{repo: repo})

	log.Println("🚀 FoodRush Catalog Service (Go) escuchando en :50051")
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("❌ Error al servir gRPC: %v", err)
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
		log.Printf("⏳ [%d/5] Esperando a que la DB esté lista...", i)
		time.Sleep(2*time.Second + time.Duration(rand.Intn(300))*time.Millisecond)
	}
	return nil, err
}
