package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
)

func main() {
	// AWS_PROFILE=perkbox-development go run cmd/ddb/main.go
	// sess := session.Must(session.NewSessionWithOptions(session.Options{
	// 	SharedConfigState: session.SharedConfigEnable,
	// }))

	// Use your ~/.aws/credentials file to get profile
	// sess := session.Must(session.NewSession(&aws.Config{
	// 	Region:      aws.String("eu-west-1"),
	// 	Credentials: credentials.NewSharedCredentials("~/.aws/credentials", "perkbox-dev"),
	// }))

	sess := session.Must(session.NewSession())

	// Create DynamoDB client
	svc := dynamodb.New(sess, &aws.Config{
		Endpoint:    aws.String("http://dynamodb:8000"),
		Region:      aws.String("eu-west-1"),
		Credentials: credentials.NewStaticCredentials("blah", "blah", ""), // AKID, SECRET_KEY, TOKEN
	})

	//listTables(svc)
	getItems(svc)

}

func listTables(svc *dynamodb.DynamoDB) error {
	input := &dynamodb.ListTablesInput{}
	fmt.Printf("Tables:\n")

	for {
		// Get the list of tables
		result, err := svc.ListTables(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case dynamodb.ErrCodeInternalServerError:
					fmt.Println(dynamodb.ErrCodeInternalServerError, aerr.Error())
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
			return nil
		}

		for _, n := range result.TableNames {
			fmt.Println(*n)
		}

		// assign the last read tablename as the start for our next call to the ListTables function
		// the maximum number of table names returned in a call is 100 (default), which requires us to make
		// multiple calls to the ListTables function to retrieve all table names
		input.ExclusiveStartTableName = result.LastEvaluatedTableName

		if result.LastEvaluatedTableName == nil {
			break
		}
	}

	return nil
}

// Item dynamoDB
type Item struct {
	UUID   string
	Status string
	Owner  string
}

func getItems(svc *dynamodb.DynamoDB) error {
	tableName := "dev__Perks_Saga"

	// Create the Expression to fill the input struct with.
	// Get all movies in that year; we'll pull out those with a higher rating later
	// filt := expression.Name("Year").Equal(expression.Value(year))

	// Or we could get by ratings and pull out those with the right year later
	filt := expression.Name("status").Equal(expression.Value("STARTED"))

	// Get back the title, year, and rating
	proj := expression.NamesList(expression.Name("uuid"), expression.Name("status"), expression.Name("owner"))

	expr, err := expression.NewBuilder().WithFilter(filt).WithProjection(proj).Build() // .WithFilter(filt)
	if err != nil {
		fmt.Println("Got error building expression:")
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Build the query input parameters
	params := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}

	// Make the DynamoDB Query API call
	result, err := svc.Scan(params)
	if err != nil {
		fmt.Println("Query API call failed:")
		fmt.Println((err.Error()))
		os.Exit(1)
	}

	numItems := 0

	for _, i := range result.Items {
		item := Item{}

		err = dynamodbattribute.UnmarshalMap(i, &item)

		if err != nil {
			fmt.Println("Got error unmarshalling:")
			fmt.Println(err.Error())
			os.Exit(1)
		}

		numItems++

		fmt.Println("UUID: ", item.UUID)
		fmt.Println("Status:", item.Status)
		fmt.Println("Owner:", item.Owner)
		fmt.Println()
	}

	fmt.Println("Found", numItems, " in the table")

	return nil
}
