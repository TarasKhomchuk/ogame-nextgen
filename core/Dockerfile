FROM rust:1.75-alpine

# Install build essentials and protoc required for gRPC code generation
RUN apk add --no-cache musl-dev protoc protobuf-dev git

WORKDIR /app

# Pre-compile dependencies architecture by creating a dummy shell
RUN cargo new --bin core-service
WORKDIR /app/core-service

COPY Cargo.toml ./
# This builds dependencies and caches them
RUN cargo build || true 

# Copy real source code
COPY . .

# Expose gRPC port
EXPOSE 50051

CMD ["cargo", "run"]
