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
		ComercioId: "mcdonalds-centro",
		Items: []*pb.OrderItem{
			{ProductoId: "hamburguesa", Cantidad: 2},
			{ProductoId: "papas", Cantidad: 1},
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
