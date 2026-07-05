package main

import (
	"context"
	"log"
	"time"

	pb "foodrush/orders/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Conectar al servidor gRPC
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("No se pudo conectar: %v", err)
	}
	defer conn.Close()
	
	client := pb.NewOrderServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	log.Println("Enviando petición CreateOrder...")
	res, err := client.CreateOrder(ctx, &pb.CreateOrderRequest{
		UserId:     "usuario-prueba-123",
		ComercioId: "c1111111-1111-1111-1111-111111111111",
		Items: []*pb.OrderItem{
			{ProductoId: "11111111-1111-1111-1111-111111111111", Cantidad: 2}, // Hamburguesa Demo ($10.00)
			{ProductoId: "22222222-2222-2222-2222-222222222222", Cantidad: 1}, // Bebida Demo ($2.50)
		},
	})

	if err != nil {
		log.Fatalf("Error al crear la orden: %v", err)
	}

	log.Printf("✅ Orden creada con éxito!")
	log.Printf("   ID de la Orden: %s", res.Id)
	log.Printf("   Total a Pagar: $%.2f", res.Total)
	log.Printf("   Estado Inicial: %s", res.Status)
}
