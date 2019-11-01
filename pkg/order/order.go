package order

import (
	"context"
	"fmt"

	"go-grpc-kubernetes/pkg/ddbstore"
	pb "go-grpc-kubernetes/proto/orderservice"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

const (
	tableName = "orders-api-dev"
)

// Server type definition
type Server struct {
	DdbSession *session.Session
}

// EnsureDDB ensure ddb exist in dev sandbox
func (s *Server) EnsureDDB() error {
	ddbClient := dynamodb.New(s.DdbSession)
	ddbDesc, err := ddbClient.DescribeTable(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		input := &dynamodb.CreateTableInput{
			AttributeDefinitions: []*dynamodb.AttributeDefinition{
				{
					AttributeName: aws.String("uuid"),
					AttributeType: aws.String("S"),
				},
			},
			// KeySchema with instance_id as hash key
			KeySchema: []*dynamodb.KeySchemaElement{
				{
					AttributeName: aws.String("uuid"),
					KeyType:       aws.String("HASH"),
				},
			},
			ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(10),
				WriteCapacityUnits: aws.Int64(10),
			},
			TableName: aws.String(tableName),
			StreamSpecification: &dynamodb.StreamSpecification{
				StreamEnabled:  aws.Bool(true),
				StreamViewType: aws.String("NEW_AND_OLD_IMAGES"),
			},
		}
		_, err := ddbClient.CreateTable(input)
		if err != nil {
			panic(err)
		}
	}
	fmt.Printf("dynamodb table description: %v\n", ddbDesc.String())
	return nil
}

// MakeServer returns a new server satisfying todo grpc service
func MakeServer() *Server {
	// This configuration is for local only
	// server := &Server{
	// 	DdbSession: session.Must(session.NewSession(&aws.Config{
	// 		Endpoint:    aws.String("http://dynamodb:8000"),
	// 		Region:      aws.String("eu-west-1"),
	// 		Credentials: credentials.NewStaticCredentials("blah", "blah", ""), // AKID, SECRET_KEY, TOKEN
	// 	})),
	// }

	server := &Server{
		DdbSession: session.Must(session.NewSession()),
	}

	if err := server.EnsureDDB(); err != nil {
		panic(err)
	}
	return server
}

// CreateOrder service
func (s *Server) CreateOrder(ctx context.Context, in *pb.Order) (*pb.Order, error) {
	out, err := ddbstore.PutProtoToDdb(in, in.GetUuid(), s.DdbSession, tableName)
	if err != nil {
		return nil, err
	}
	return out.(*pb.Order), nil
}

// UpdateOrder service
func (s *Server) UpdateOrder(ctx context.Context, in *pb.Order) (*pb.Order, error) {
	out, err := ddbstore.PutProtoToDdb(in, in.GetUuid(), s.DdbSession, "orders-api-dev")
	if err != nil {
		return nil, err
	}
	return out.(*pb.Order), nil
}

// GetOrder service
func (s *Server) GetOrder(ctx context.Context, in *pb.RequestBy) (*pb.Order, error) {
	out, err := ddbstore.GetProtoFromDdb(in, in.GetUuid(), s.DdbSession, "orders-api-dev")
	if err != nil {
		return nil, err
	}
	return out.(*pb.Order), nil
}

// DeleteOrder service
func (s *Server) DeleteOrder(ctx context.Context, in *pb.RequestBy) (*pb.Order, error) {
	err := ddbstore.DeleteProtoFromDdb(in.GetUuid(), s.DdbSession, "orders-api-dev")
	if err != nil {
		return nil, err
	}
	return &pb.Order{Uuid: in.GetUuid()}, nil
}
