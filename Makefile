.PHONY: 


proto: 
	protoc --proto_path=proto --proto_path=/home/lukas/go/src/github.com/golang/protobuf/protoc-gen-go/ \
	--go_out=plugins=grpc:proto service.proto

# Before docker operations
login-ecr:
	$(aws ecr get-login --no-include-email --profile perkbox-dev)

clean:
	rm -f cmd/grpc-server/grpc-server

kubenamespace:
	kubectl delete namespace lukas
	kubectl create namespace lukas
	kubectl annotate namespace lukas linkerd.io/inject=enabled

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" \
	-o ./cmd/grpc-server/grpc-server ./cmd/grpc-server/*.go

dev-docker: clean build
	docker build -t orderapi-grpc-server cmd/grpc-server && \
	docker tag orderapi-grpc-server 053675267868.dkr.ecr.eu-west-1.amazonaws.com/lukas/orderapi-grpc-server && \
	docker push 053675267868.dkr.ecr.eu-west-1.amazonaws.com/lukas/orderapi-grpc-server

deploy-dev: kubenamespace dev-docker 
	kubectl apply -f ./kubernetes/dev/

linkerd-profile:
	#rm -rf ./kubernetes/profile-*.yaml
	linkerd profile --proto ./proto/todoapiservice/todoapiservice.proto todoapi-grpc-server > ./kubernetes/profile-todoapi-grpc-server.yaml
	go run cmd/postserviceprofile/main.go -in kubernetes/profile-todoapi-grpc-server.yaml -ns todo -out kubernetes/dev/profile-todoapi-grpc-server.yaml