# gateway/Dockerfile
FROM golang:1.22-alpine

# Install build essentials, protoc and standard protobuf tools
RUN apk add --no-cache musl-dev protoc protobuf-dev git

# Install Go specific plugins for Protocol Buffers architecture
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0

WORKDIR /app

EXPOSE 8080

# Execute generation script first, then run the gateway application
CMD ["sh", "-c", "go generate ./... && go run ."]
