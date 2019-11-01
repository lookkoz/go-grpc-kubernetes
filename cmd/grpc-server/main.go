package main

import (
	"go-grpc-kubernetes/pkg/order"
	pb "go-grpc-kubernetes/proto/orderservice"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	if err := runServer(); err != nil {
		panic(err)
	}
}

func runServer() error {
	lis, err := net.Listen("tcp", ":9092")
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)

	server := order.MakeServer()
	pb.RegisterOrderServiceServer(grpcServer, server)

	if err := grpcServer.Serve(lis); err != nil {
		return err
	}

	return nil
}
