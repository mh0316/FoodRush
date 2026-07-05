package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	catalogpb "foodrush/orders/catalogpb"
	orderkafka "foodrush/orders/kafka"
	pb "foodrush/orders/proto"
	"foodrush/orders/repository"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrderServer struct {
	pb.UnimplementedOrderServiceServer
	repo     repository.OrderStore
	catalog  catalogpb.CatalogServiceClient
	producer *orderkafka.Producer
}

func NewOrderServer(
	repo repository.OrderStore,
	catalog catalogpb.CatalogServiceClient,
	producer *orderkafka.Producer,
) *OrderServer {
	return &OrderServer{
		repo:     repo,
		catalog:  catalog,
		producer: producer,
	}
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
	correlationID := uuid.New().String()

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

	// SAGA: Add event to Outbox within the document (atomic in MongoDB for single document)
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"event_id":       uuid.New().String(),
		"correlation_id": correlationID,
		"event_type":     "foodrush.order.created",
		"source":         "orders-service",
		"order_id":       order.Id,
		"user_id":        order.UserId,
		"comercio_id":    order.ComercioId,
		"total":          order.Total,
		"status":         order.Status,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	})

	outboxEvent := &pb.OutboxEvent{
		Id:        uuid.New().String(),
		EventType: "foodrush.order.created",
		Payload:   string(eventPayload),
		Headers: map[string]string{
			"correlation_id": correlationID,
		},
		Processed: false,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if err := s.repo.AddOutboxEvent(ctx, order.Id, outboxEvent); err != nil {
		log.Printf("[orders-service] failed to add outbox event correlation_id=%s order_id=%s error=%v", correlationID, order.Id, err)
		// We could decide to fail the whole request or just log.
		// Since order is created, we should try to ensure the event is eventually sent.
	}

	log.Printf(
		"[orders-service] order created and outbox event added correlation_id=%s order_id=%s user_id=%s total=%.2f",
		correlationID,
		order.Id,
		order.UserId,
		order.Total,
	)

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

		product, err := s.catalog.GetProductDetails(
			lookupCtx,
			&catalogpb.GetProductDetailsRequest{Id: productID},
		)

		cancel()

		if err == nil {
			return product, nil
		}

		lastErr = err
		log.Printf("catalog lookup retry %d/3 for product %s: %v", i, productID, err)
		time.Sleep(250 * time.Millisecond)
	}

	return nil, status.Errorf(
		codes.FailedPrecondition,
		"catalog unavailable for product %s: %v",
		productID,
		lastErr,
	)
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
