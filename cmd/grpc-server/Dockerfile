FROM alpine:latest

RUN mkdir /app
WORKDIR /app
ADD grpc-server /app
EXPOSE 9092
ENTRYPOINT ["./grpc-server"]