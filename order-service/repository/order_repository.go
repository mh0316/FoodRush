package repository

import (
	"context"

	pb "foodrush/orders/proto"
)

type OrderStore interface {
	CreateOrder(ctx context.Context, order *pb.Order) error
	GetOrder(ctx context.Context, id string) (*pb.Order, error)
	UpdateOrderStatus(ctx context.Context, qrRetiro string, status string) (*pb.Order, error)
}
