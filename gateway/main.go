package main

import (
	"fmt"
	"time"
)

func main() {
	// Temporary boot loop until we add WebSocket and gRPC code
	fmt.Println("Gateway service started successfully in Docker!")
	for {
		time.Sleep(1 * time.Hour)
	}
}
