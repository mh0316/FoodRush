package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	_ "github.com/lib/pq" // Driver de Postgres
	pb "github.com/mh0316/catalog/pb"
	"google.golang.org/grpc"
)

// Estructura del servidor con la conexión a la BD
type server struct {
	pb.UnimplementedCatalogServiceServer
	db *sql.DB
}

// Este método consulta la tabla 'comercios'
func (s *server) ListComercios(ctx context.Context, req *pb.ListComerciosRequest) (*pb.ListComerciosResponse, error) {
	log.Printf("📥 ListComercios: SoloActivos=%v", req.SoloActivos)

	query := "SELECT id, nombre, direccion, activo FROM comercios"
	if req.SoloActivos {
		query += " WHERE activo = true"
	}

	rows, err := s.db.Query(query)
	if err != nil {
		log.Printf("❌ Error en ListComercios: %v", err)
		return nil, err
	}
	defer rows.Close()

	var comercios []*pb.Comercio
	for rows.Next() {
		c := &pb.Comercio{}
		if err := rows.Scan(&c.Id, &c.Nombre, &c.Direccion, &c.Activo); err != nil {
			return nil, err
		}
		comercios = append(comercios, c)
	}

	return &pb.ListComerciosResponse{Comercios: comercios}, nil
}

// Este método consulta la tabla 'productos' por ID de comercio
func (s *server) GetMenuByComercio(ctx context.Context, req *pb.GetMenuByComercioRequest) (*pb.GetMenuByComercioResponse, error) {
	log.Printf("📥 GetMenuByComercio: ID=%s", req.ComercioId)

	rows, err := s.db.Query("SELECT id, nombre, precio, comercio_id, disponible FROM productos WHERE comercio_id = $1", req.ComercioId)
	if err != nil {
		log.Printf("❌ Error en GetMenuByComercio: %v", err)
		return nil, err
	}
	defer rows.Close()

	var productos []*pb.Product
	for rows.Next() {
		p := &pb.Product{}
		if err := rows.Scan(&p.Id, &p.Nombre, &p.Precio, &p.ComercioId, &p.Disponible); err != nil {
			return nil, err
		}
		productos = append(productos, p)
	}

	return &pb.GetMenuByComercioResponse{Productos: productos}, nil
}

// Este método consulta la tabla 'productos' por ID de producto
func (s *server) GetProductDetails(ctx context.Context, req *pb.GetProductDetailsRequest) (*pb.Product, error) {
	log.Printf("📥 GetProductDetails: ID=%s", req.Id)

	p := &pb.Product{}
	err := s.db.QueryRow("SELECT id, nombre, precio, comercio_id, disponible FROM productos WHERE id = $1", req.Id).
		Scan(&p.Id, &p.Nombre, &p.Precio, &p.ComercioId, &p.Disponible)

	if err != nil {
		log.Printf("❌ Error en GetProductDetails: %v", err)
		return nil, fmt.Errorf("producto no encontrado: %v", err)
	}

	return p, nil
}

// Esta función main inicializa la conexión a la BD y el servidor gRPC.
// Agrega una lógica de reintento para asegurar que el servicio se inicie solo cuando la BD esté lista.
func main() {
	// Se obtiene la configuración de conexión (Docker usa variables de entorno)
	connStr := os.Getenv("DB_URL")
	if connStr == "" {
		connStr = "user=postgres password=admin123 host=localhost port=5432 dbname=foodrush_db sslmode=disable"
	}

	// Se conecta a PostgreSQL con reintentos (Wait-for-DB logic)
	var db *sql.DB
	var err error
	for i := 1; i <= 5; i++ {
		db, err = sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
		}

		if err == nil {
			break
		}

		log.Printf("⏳ [%d/5] Esperando a que la DB esté lista...", i)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		log.Fatalf("❌ No se pudo conectar a la DB tras varios intentos: %v", err)
	}
	defer db.Close()
	log.Println("✅ Conexión exitosa a PostgreSQL")

	// Se inicia el servidor gRPC
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Error al abrir puerto: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterCatalogServiceServer(srv, &server{db: db})

	log.Println("🚀 FoodRush Catalog Service (Go) escuchando en :50051")
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("❌ Error al servir gRPC: %v", err)
	}
}
