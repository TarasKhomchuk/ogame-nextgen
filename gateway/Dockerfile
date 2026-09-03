FROM golang:1.22-alpine

WORKDIR /app

# Install git if needed for dependencies
RUN apk add --no-cache git

# Copy dependency files first for caching
COPY go.mod ./
RUN go mod download

# Copy the source code
COPY . .

# Expose port for WebSockets
EXPOSE 8080

# Run with live reloading using built-in go run
CMD ["go", "run", "main.go"]
