package ddbstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/sha1sum/aws_signing_client"

	"github.com/elastic/go-elasticsearch/v7"
)

var (
	ErrHashKeyNotFound = errors.New("hash key not found")
	slim               = regexp.MustCompile(`\s+`)
)

// GetEnv return env variable if key existed or returning the fallback
func GetEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// DynamoDetails is a wrapper around the DynamoDBAPI interface,
// it defines behavior for accessing DynamoDB Table metadata
type DynamoDetails struct {
	dynamodbiface.DynamoDBAPI
}

// Details is the data needed to index the most recent dataoff to elasticsearch
type Details struct {
	HashKey, RangeKey, TableName string
}

// GetFromKeys return the details for ES from given hash and range keys
func (d *DynamoDetails) GetFromKeys(tableName string, hashKey, rangeKey string) (details *Details, err error) {
	if hashKey == "" {
		return nil, ErrHashKeyNotFound
	}
	details = &Details{
		TableName: tableName,
		HashKey:   hashKey,
		RangeKey:  rangeKey,
	}
	return
}

// Get Extracts out the attribute Value of Hash Key and Range key from the describe table output
func (d *DynamoDetails) Get(tableName string) (details *Details, err error) {
	var out *dynamodb.DescribeTableOutput
	req, out := d.DescribeTableRequest(&dynamodb.DescribeTableInput{
		TableName: aws.String(tableName),
	})
	if err = req.Send(); err != nil {
		return nil, err
	}
	// We NEED a hash key to uniquely identify records
	hashKey := findAttributeByKeyType(out.Table.KeySchema, "HASH")
	if hashKey == "" {
		return nil, ErrHashKeyNotFound
	}
	// range keys are nice but we don't necessarily need one to uniquely identify a Dynamo Record
	var rangeKey string
	r := findAttributeByKeyType(out.Table.KeySchema, "RANGE")
	rangeKey = r
	details = &Details{
		TableName: tableName,
		HashKey:   hashKey,
		RangeKey:  rangeKey,
	}
	return
}

// findAttributeByKeyType walk through ddb key schema element and return one that matching keyType
func findAttributeByKeyType(schema []*dynamodb.KeySchemaElement, keyType string) string {
	for _, element := range schema {
		if *element.KeyType == keyType {
			return *element.AttributeName
		}
	}
	return ""
}

// docType return automated doc type from hash or hash with range key
func (d *Details) docType() string {
	if d.RangeKey != "" {
		return fmt.Sprintf("%s-%s", d.HashKey, d.RangeKey)
	}
	return d.HashKey
}

// docID return automated unique doc id from hash or hash with range key
func (d *Details) docID(item map[string]events.DynamoDBAttributeValue) (id string) {
	if d != nil {
		if d.RangeKey != "" {
			id = fmt.Sprintf("%s-%s", item[d.HashKey].String(), item[d.RangeKey].String())
		} else {
			id = item[d.HashKey].String()
		}
	}
	return id
}

// index return automated unique ddb table name as elasticsearch index
func (d *Details) index() string {
	return strings.ToLower(d.TableName)
}

// EventStreamToMap to convert inconsistent ddb stream to ddb attrs
func EventStreamToMap(attribute interface{}) map[string]*dynamodb.AttributeValue {
	m := make(map[string]*dynamodb.AttributeValue)
	tmp := make(map[string]events.DynamoDBAttributeValue)
	switch t := attribute.(type) {
	case map[string]events.DynamoDBAttributeValue:
		tmp = t
	case events.DynamoDBAttributeValue:
		tmp = t.Map()
	}
	for k, v := range tmp {
		switch v.DataType() {
		case events.DataTypeString:
			s := v.String()
			m[k] = &dynamodb.AttributeValue{
				S: &s,
			}
		case events.DataTypeBoolean:
			b := v.Boolean()
			m[k] = &dynamodb.AttributeValue{
				BOOL: &b,
			}
		case events.DataTypeMap:
			m[k] = &dynamodb.AttributeValue{
				M: EventStreamToMap(v),
			}
		case events.DataTypeNumber:
			n := v.Number()
			m[k] = &dynamodb.AttributeValue{
				N: &n,
			}
		}
	}
	return m
}

// AWSSigningTransport for signer awsv4 with Elasticsearch
type AWSSigningTransport struct {
	HTTPClient *http.Client
}

// RoundTrip implementation
func (a AWSSigningTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return a.HTTPClient.Do(req)
}

// Elasticsearch is an ES Client which will perform Elasticsearch Updates for Dynamo Items
type Elasticsearch struct {
	*elasticsearch.Client
}

// GetSession return aws session with appropriate credentials
func GetSession() (*session.Session, error) {
	return session.NewSession(&aws.Config{
		Region: aws.String(GetEnv("AWS_REGION", GetEnv("REGION", "us-east-1"))),
	})
}

// NewElasticsearch return a new Elasticsearch client instance
// with AWS v4 signer from default aws session
func NewElasticsearch() (*Elasticsearch, error) {
	sess, err := GetSession()
	if err != nil {
		return nil, err
	}
	return NewElasticsearchWithSession(sess)
}

// NewElasticsearchWithSession return a new Elasticsearch client instance
// with AWS v4 signer from provided session
func NewElasticsearchWithSession(sess *session.Session) (*Elasticsearch, error) {
	signer := v4.NewSigner(sess.Config.Credentials)
	awsclient, err := aws_signing_client.New(signer, nil, "es", *sess.Config.Region)
	if err != nil {
		return nil, err
	}
	signingTransport := AWSSigningTransport{
		HTTPClient: awsclient,
	}
	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Transport: http.RoundTripper(signingTransport),
	})
	if err != nil {
		return nil, err
	}
	esclient := new(Elasticsearch)
	esclient.Client = es
	return esclient, nil
}

// Update takes a reference to a dstream.Details object;
// which is used to figure out which Elasticsearch Index to update;
// And an item map[string]events.DynamoDBAttributeValue which will be turned into JSON
// then indexed into Elasticsearch
func (es *Elasticsearch) Update(d *Details, item map[string]events.DynamoDBAttributeValue) error {
	res, err := es.Info()
	if err != nil {
		return err
	}
	defer res.Body.Close()
	fmt.Printf("ddbstore:elasticsearch:Update: %v\n", slim.ReplaceAllString(res.String(), " "))
	tmp := EventStreamToMap(item)
	var i interface{}
	if err := dynamodbattribute.UnmarshalMap(tmp, &i); err != nil {
		return err
	}
	body, err := json.Marshal(i)
	if err != nil {
		return err
	}
	res, err = es.Index(
		d.index(),
		strings.NewReader(string(body)),
		es.Index.WithRefresh("true"),
		es.Index.WithDocumentID(d.docID(item)),
		es.Index.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.New(fmt.Sprintf("[%v] Error indexing document ID=%v", res.Status(), d.docID(item)))
	}
	fmt.Printf("ddbstore:elasticsearch:Update: %v\n", slim.ReplaceAllString(res.String(), " "))
	return nil
}

// Remove removes index from reference of dstream.Details object
func (es *Elasticsearch) Remove(d *Details, item map[string]events.DynamoDBAttributeValue) error {
	res, err := es.Info()
	if err != nil {
		return err
	}
	tmp := EventStreamToMap(item)
	var i interface{}
	if err := dynamodbattribute.UnmarshalMap(tmp, &i); err != nil {
		return err
	}
	res, err = es.Delete(
		d.index(),
		d.docID(item),
		es.Delete.WithRefresh("true"),
		es.Delete.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.New(fmt.Sprintf("[%v] Error removing document ID=%v", res.Status(), d.docID(item)))
	}
	fmt.Printf("ddbstore:elasticsearch:Remove: %v\n", slim.ReplaceAllString(res.String(), " "))
	return nil
}

// BuildQuery for elasticsearch client
func (es *Elasticsearch) BuildQuery(query string, after ...string) io.Reader {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(query)
	if len(after) > 0 && after[0] != "" && after[0] != "null" {
		b.WriteString(",\n")
		b.WriteString(fmt.Sprintf(`	"search_after": %s`, after))
	}
	b.WriteString("\n}")
	fmt.Printf("%s\n", b.String())
	return strings.NewReader(b.String())
}

type SearchResults struct {
	Total int         `json:"total"`
	Hits  interface{} `json:"hits"`
}

// Query for elasticsearch client
func (es *Elasticsearch) Query(d *Details, query, sort string, size int, result interface{}) error {
	res, err := es.Search(
		es.Search.WithIndex(d.index()),
		es.Search.WithQuery(query),
		es.Search.WithSort(sort),
		es.Search.WithSize(size),
		es.Search.WithContext(context.Background()),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return errors.New(fmt.Sprintf("[%v] Error seaching query=(%v) sort=(%v) size=(%v)", res.Status(), query, sort, size))
	}
	decoder := json.NewDecoder(res.Body)
	return decoder.Decode(&result)
}
