package handlers

import (
	"context"
	orderservice "go-grpc-kubernetes/proto/orderservice"
)

// OrderServiceServer server type definition
type OrderServiceServer struct{}

// MakeServer returns a new server
func MakeServer() *OrderServiceServer {
	server := &OrderServiceServer{}
	return server
}

// CreateOrder service
func (*OrderServiceServer) CreateOrder(ctx context.Context, in *orderservice.Order) (*orderservice.Order, error) {
	return &orderservice.Order{
		Uuid:        in.Uuid,
		ProductUuid: in.ProductUuid,
		Quantity:    10,
		Amount:      2.50,
		Currency:    "PLN",
		Status:      0,
		Timestamp:   1218731821,
	}, nil
}

// UpdateOrder service
func (*OrderServiceServer) UpdateOrder(ctx context.Context, in *orderservice.Order) (*orderservice.Order, error) {
	return &orderservice.Order{
		Uuid:        in.Uuid,
		ProductUuid: in.ProductUuid,
		Quantity:    in.Quantity + 2,
		Amount:      in.Amount + 10,
		Currency:    in.Currency,
		Status:      in.Status,
		Timestamp:   in.Timestamp,
	}, nil
}

// GetOrder service
func (*OrderServiceServer) GetOrder(ctx context.Context, in *orderservice.RequestBy) (*orderservice.Order, error) {
	return &orderservice.Order{
		Uuid:        "23123",
		ProductUuid: "1231231",
		Quantity:    10,
		Amount:      2.50,
		Currency:    "PLN",
		Status:      0,
		Timestamp:   1218731821,
	}, nil
}
