package server

import (
	"context"
	"log"

	"github.com/google/uuid"
	pb "foodrush/orders/proto"
	"foodrush/orders/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderServer struct {
	pb.UnimplementedOrderServiceServer
	repo *repository.MongoDB
}

func NewOrderServer(repo *repository.MongoDB) *OrderServer {
	return &OrderServer{repo: repo}
}

func (s *OrderServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	log.Printf("CreateOrder request: user_id=%s, comercio_id=%s", req.UserId, req.ComercioId)

	if req.UserId == "" || req.ComercioId == "" || len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}

	var total float64
	for _, item := range req.Items {
		total += float64(item.Cantidad) * 10.0 // Mock price
	}

	orderId := uuid.New().String()
	qrRetiro := "QR_" + uuid.New().String()
	
	order := &pb.Order{
		Id:             orderId,
		UserId:         req.UserId,
		ComercioId:     req.ComercioId,
		Items:          req.Items,
		Total:          total,
		Status:         "CREATED",
		QrRetiro:       qrRetiro,
		ComprobanteUrl: "https://foodrush.com/receipts/" + orderId,
	}

	err := s.repo.CreateOrder(ctx, order)
	if err != nil {
		log.Printf("Failed to insert order: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to create order")
	}

	return &pb.CreateOrderResponse{
		Id:     order.Id,
		Total:  order.Total,
		Status: order.Status,
	}, nil
}

func (s *OrderServer) GetOrderDetails(ctx context.Context, req *pb.GetOrderDetailsRequest) (*pb.Order, error) {
	log.Printf("GetOrderDetails request: id=%s", req.Id)

	order, err := s.repo.GetOrder(ctx, req.Id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "order not found")
	}

	return order, nil
}

func (s *OrderServer) ConfirmOrderPickup(ctx context.Context, req *pb.ConfirmOrderPickupRequest) (*pb.ConfirmOrderPickupResponse, error) {
	log.Printf("ConfirmOrderPickup request: qr_retiro=%s", req.QrRetiro)

	order, err := s.repo.UpdateOrderStatus(ctx, req.QrRetiro, "DELIVERED")
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "order not found or invalid qr")
	}

	return &pb.ConfirmOrderPickupResponse{
		Id:          order.Id,
		NuevoStatus: order.Status,
	}, nil
}
