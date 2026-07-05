package server

import (
	"context"
	"time"

	catalogpb "foodrush/orders/catalogpb"
	"google.golang.org/grpc"
)

type ResilientCatalogClient struct {
	client catalogpb.CatalogServiceClient
	cb     *CircuitBreaker
}

func NewResilientCatalogClient(client catalogpb.CatalogServiceClient, maxFailures int, cooldown time.Duration) *ResilientCatalogClient {
	return &ResilientCatalogClient{
		client: client,
		cb:     NewCircuitBreaker(maxFailures, cooldown),
	}
}

func (r *ResilientCatalogClient) GetProductDetails(ctx context.Context, in *catalogpb.GetProductDetailsRequest, opts ...grpc.CallOption) (*catalogpb.Product, error) {
	res, err := r.cb.Execute(func() (interface{}, error) {
		return r.client.GetProductDetails(ctx, in, opts...)
	})
	if err != nil {
		return nil, err
	}
	return res.(*catalogpb.Product), nil
}

func (r *ResilientCatalogClient) ListComercios(ctx context.Context, in *catalogpb.ListComerciosRequest, opts ...grpc.CallOption) (*catalogpb.ListComerciosResponse, error) {
	return r.client.ListComercios(ctx, in, opts...)
}

func (r *ResilientCatalogClient) GetMenuByComercio(ctx context.Context, in *catalogpb.GetMenuByComercioRequest, opts ...grpc.CallOption) (*catalogpb.GetMenuByComercioResponse, error) {
	return r.client.GetMenuByComercio(ctx, in, opts...)
}
