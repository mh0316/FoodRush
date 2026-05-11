package server

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	catalogpb "foodrush/orders/catalogpb"
	pb "foodrush/orders/proto"
	"foodrush/orders/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderServer struct {
	pb.UnimplementedOrderServiceServer
	repo    repository.OrderStore
	catalog catalogpb.CatalogServiceClient
}

func NewOrderServer(repo repository.OrderStore, catalog catalogpb.CatalogServiceClient) *OrderServer {
	return &OrderServer{repo: repo, catalog: catalog}
}

func (s *OrderServer) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	log.Printf("CreateOrder request: user_id=%s, comercio_id=%s", req.UserId, req.ComercioId)

	if req.UserId == "" || req.ComercioId == "" || len(req.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "missing required fields")
	}
	for _, item := range req.Items {
		if strings.TrimSpace(item.ProductoId) == "" || item.Cantidad <= 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid order items")
		}
	}

	var total float64
	for _, item := range req.Items {
		product, err := s.getProduct(ctx, item.ProductoId)
		if err != nil {
			return nil, err
		}
		if !product.Disponible {
			return nil, status.Error(codes.FailedPrecondition, "product unavailable")
		}
		total += float64(item.Cantidad) * product.Precio
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

func (s *OrderServer) getProduct(ctx context.Context, productID string) (*catalogpb.Product, error) {
	var lastErr error
	for i := 1; i <= 3; i++ {
		lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		product, err := s.catalog.GetProductDetails(lookupCtx, &catalogpb.GetProductDetailsRequest{Id: productID})
		cancel()
		if err == nil {
			return product, nil
		}
		lastErr = err
		log.Printf("catalog lookup retry %d/3 for product %s: %v", i, productID, err)
		time.Sleep(250 * time.Millisecond)
	}

	return nil, status.Errorf(codes.FailedPrecondition, "catalog unavailable for product %s: %v", productID, lastErr)
}

func (s *OrderServer) GetOrderDetails(ctx context.Context, req *pb.GetOrderDetailsRequest) (*pb.Order, error) {
	log.Printf("GetOrderDetails request: id=%s", req.Id)

	order, err := s.repo.GetOrder(ctx, req.Id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found")
		}
		return nil, status.Errorf(codes.Internal, "failed to get order")
	}

	return order, nil
}

func (s *OrderServer) ConfirmOrderPickup(ctx context.Context, req *pb.ConfirmOrderPickupRequest) (*pb.ConfirmOrderPickupResponse, error) {
	log.Printf("ConfirmOrderPickup request: qr_retiro=%s", req.QrRetiro)

	order, err := s.repo.UpdateOrderStatus(ctx, req.QrRetiro, "DELIVERED")
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "order not found or invalid qr")
		}
		return nil, status.Errorf(codes.Internal, "failed to update order")
	}

	return &pb.ConfirmOrderPickupResponse{
		Id:          order.Id,
		NuevoStatus: order.Status,
	}, nil
}
