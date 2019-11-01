# GO GRPC service deplyed to kubernetes with linkerd - service mesh tool
Go GRPC example with deploy to kubernetes

First define GRPC service in proto file:
proto/orderservice/orderservice.proto

Create go.mod file by command:
`go mod init go-grpc-kubernetes`

Run our GRPC server: 
`go run cmd/grpc-server/main.go`

We can check it is running with the following command:
`sudo lsof -i -P -n | grep LISTEN `

With the command you can follow packets sent to your GRPC server:
`sudo tcpdump -i any port 9092`

Install `grpcurl` tool to test our GRPC service https://github.com/fullstorydev/grpcurl.

Next you can use it with simple command:
`grpcurl -proto ./proto/orderservice/orderservice.proto -plaintext -d '{}' localhost:9092 order.api.v1.OrderService/CreateOrder`
`grpcurl -proto ./proto/orderservice/orderservice.proto -plaintext -d '{"uuid": "a75737f2-f983-11e9-82c7-63fbea64c327"}' localhost:9092 order.api.v1.OrderService/CreateOrder`

We can list all services with this command too:
`grpcurl -plaintext localhost:9092 list` # if we have service reflection on our GRPC server
`grpcurl -import-path ../protos -proto ./proto/orderservice/orderservice.proto list` # by proto definition 
`grpcurl -plaintext localhost:9092 describe order.api.v1.OrderService` # describe service


## Go connect to AWS DynamoDB with specific credentials 
Inside script `cmd/ddb/main.go` there is initiated connection to DynamoDB with aws profile defined in your configuration of `~/.aws/`.
You can connect to specific profile by setting it up inline:
`AWS_PROFILE=perkbox-development go run cmd/ddb/main.go`



You can list tables with command
`aws dynamodb list-tables --endpoint http://dynamodb:8000`

Now ready lets tests our service grpc endpoint and create some records:
`grpcurl -proto web/go/src/go-grpc-kubernetes/proto/orderservice/orderservice.proto -plaintext -d "{\"uuid\": \"$(uuid)\"}" localhost:9092 order.api.v1.OrderService/CreateOrder`

Check DynamoDB records it created `aws dynamodb scan --table orders-api-dev --endpoint-url http://dynamo:8000`



# Deploy to kubernetes
Next we gonna deploy our service to kubernetes (AWS ECR).

Before we can deploy this we need a Dockerfile and push it to the docker repository. 
In Amazon Elastic Container Registry (AWS ECR) lets create new repository, according to what we specified in our kubernetes.yaml
`spec > template > spec > containers > image`
`053675267868.dkr.ecr.eu-west-1.amazonaws.com/lukas/orderapi-grpc-server`
Our repository name should be called lukas/orderapi-grpc-server.

Next we should compile our application, build docker image and push the image to ECR:
Lets login docker to our ECR first if we haven't done that:
`$(aws ecr get-login --no-include-email --profile perkbox-dev)`

```s
rm -f cmd/grpc-server/grpc-server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ./cmd/grpc-server/grpc-server ./cmd/grpc-server/*.go
docker build -t orderapi-grpc-server cmd/grpc-server && \
docker tag orderapi-grpc-server 053675267868.dkr.ecr.eu-west-1.amazonaws.com/lukas/orderapi-grpc-server && \
docker push 053675267868.dkr.ecr.eu-west-1.amazonaws.com/lukas/orderapi-grpc-server
```

If you use many environments check first which one is currently set. Current one gonna be marked with asterisk.
`kubectl config context`

`kubectl create namespace lukas`
`kubectl annotate lukas linkerd.io/inject=enabled`

After abovee steps we should be able to see created namespace and meshed to linkerd.
`kubectl describe namespaces lukas`

Next we can deply it:
`kubectl apply -f kubernetes/dev/deployment.yaml`

Next we can see if it's success
`kubectl get pods -n lukas`

For troubleshooting we can have a look of what went wrong in logs or events:
`kubectl logs -f orderapi-grpc-server-7ddc957c9d-n7twk orderapi-grpc-server -n lukas`
`kubectl -n lukas get events -w`

Before you deploy again just remove the old namespace and start from fresh.


# Working with service deployed to kubernetes

Once we have service deployed to kubernetes we can expose port and map it t
Expose the service to the external network

`kubectl -n lukas expose deployment/orderapi-grpc-server --port 9092 --target-port 9092 --name=orderapi-grpc-server-external --type LoadBalancer`
With this command we get access to the external IP address and exposed port:
`kubectl -n lukas get svc`
Ater we are done with testing we can remove it:
`kubectl delete service orderapi-grpc-server-external -n lukas`


Before we remove it lets test our service grpc endpoint and create some records in kubernetes:
`grpcurl -proto ./proto/orderservice/orderservice.proto -plaintext -d "{\"uuid\": \"$(uuid)\"}" afc31bb00fbd611e9ac520221ef0b91a-1138963974.eu-west-1.elb.amazonaws.com:9092 order.api.v1.OrderService/CreateOrder`

Now lets have a look at linkerd dashboard.
`linkerd dashboard`

Lets run some more requests to see how it acts on linkerd and grafana:
`for ((i=0; i<99; i++)); do grpcurl -proto ./proto/orderservice/orderservice.proto -plaintext -d "{\"uuid\": \"$(uuid)\"}" afc31bb00fbd611e9ac520221ef0b91a-1138963974.eu-west-1.elb.amazonaws.com:9092 order.api.v1.OrderService/CreateOrder; done`