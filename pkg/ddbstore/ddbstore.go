package ddbstore

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

var (
	ErrItemNotFound  = errors.New("item not found")
	ErrKeyMismatched = errors.New("key mismatched")
)

func verifyProto(in proto.Message, instanceID string) bool {
	if instanceID == "" {
		return false
	}
	return true
}

// GetProtoFromDdb get item from dynamodb directly and parse to proto message
func GetProtoFromDdb(in proto.Message, instanceID string, ddbSession *session.Session, tableName string) (proto.Message, error) {
	if !verifyProto(in, instanceID) {
		return nil, ErrKeyMismatched
	}
	ddbClient := dynamodb.New(ddbSession)
	input := &dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"uuid": {
				S: aws.String(instanceID),
			},
		},
		TableName: aws.String(tableName),
	}
	output, err := ddbClient.GetItem(input)
	if err != nil {
		return nil, err
	}
	if len(output.Item) == 0 {
		return nil, ErrItemNotFound
	}
	mOut := &map[string]interface{}{}
	err = dynamodbattribute.UnmarshalMap(output.Item, mOut)
	if err != nil {
		return nil, err
	}
	bOut, err := json.Marshal(mOut)
	if err != nil {
		return nil, err
	}
	out := proto.Clone(in)
	wOut := bytes.NewReader(bOut)
	err = jsonpb.Unmarshal(wOut, out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PutProtoToDdb put item to dynamodb directly from proto message
func PutProtoToDdb(in proto.Message, instanceID string, ddbSession *session.Session, tableName string) (proto.Message, error) {
	out, err := GetProtoFromDdb(in, instanceID, ddbSession, tableName)
	if err == nil {
		// item with key already existed, we need to merge the payload
		proto.Merge(out, in)
	} else if err == ErrKeyMismatched {
		return nil, err
	} else {
		out = proto.Clone(in)
	}
	var bIn []byte
	wIn := bytes.NewBuffer(bIn)
	marshaller := new(jsonpb.Marshaler)
	marshaller.OrigName = true
	err = marshaller.Marshal(wIn, out)
	if err != nil {
		return nil, err
	}
	mIn := &map[string]interface{}{}
	err = json.Unmarshal(wIn.Bytes(), mIn)
	if err != nil {
		return nil, err
	}
	attrs, err := dynamodbattribute.MarshalMap(mIn)
	if err != nil {
		return nil, err
	}
	input := &dynamodb.PutItemInput{
		Item:      attrs,
		TableName: aws.String(tableName),
	}
	ddbClient := dynamodb.New(ddbSession)
	_, err = ddbClient.PutItem(input)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BatchPutProtoToDdb put items to dynamodb in batch directly from proto message
func BatchPutProtoToDdb(ins []proto.Message, ddbSession *session.Session, tableName string) error {
	ddbclient := dynamodb.New(ddbSession)
	inputs := make([]*dynamodb.WriteRequest, 0)
	for _, req := range ins {
		var bIn []byte
		wIn := bytes.NewBuffer(bIn)
		marshaller := new(jsonpb.Marshaler)
		marshaller.OrigName = true
		err := marshaller.Marshal(wIn, req)
		if err != nil {
			return err
		}
		mIn := &map[string]interface{}{}
		err = json.Unmarshal(wIn.Bytes(), mIn)
		if err != nil {
			return err
		}
		attrs, err := dynamodbattribute.MarshalMap(mIn)
		if err != nil {
			return err
		}
		input := &dynamodb.WriteRequest{
			PutRequest: &dynamodb.PutRequest{
				Item: attrs,
			},
		}
		inputs = append(inputs, input)
		if len(inputs) >= 25 {
			batchInput := &dynamodb.BatchWriteItemInput{}
			batchInput.SetRequestItems(map[string][]*dynamodb.WriteRequest{
				tableName: inputs,
			})
			_, err := ddbclient.BatchWriteItem(batchInput)
			if err != nil {
				return err
			}
			inputs = make([]*dynamodb.WriteRequest, 0)
		}
	}
	if len(inputs) > 0 {
		batchInput := &dynamodb.BatchWriteItemInput{}
		batchInput.SetRequestItems(map[string][]*dynamodb.WriteRequest{
			tableName: inputs,
		})
		_, err := ddbclient.BatchWriteItem(batchInput)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteProtoFromDdb remove item from dynamodb directly from instance meta
func DeleteProtoFromDdb(instanceID string, ddbSession *session.Session, tableName string) error {
	ddbClient := dynamodb.New(ddbSession)
	input := &dynamodb.DeleteItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"uuid": {
				S: aws.String(instanceID),
			},
		},
		TableName: aws.String(tableName),
	}
	_, err := ddbClient.DeleteItem(input)
	return err
}
