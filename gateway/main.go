package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	// Import the auto-generated proto package
	// pb "ogame-nextgen/gateway/proto"
)

//go:generate protoc --proto_path=/proto --go_out=. --go-grpc_out=. /proto/game.proto

var upgrader = websocket.Upgrader{
	// Allow all origins for local development phase 1 testing
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Global gRPC client reference connecting to Rust Core
var gameClient GameServiceClient

// Incoming packet schema from frontend
type ClientPacket struct {
	Action string          `json:"action"`
	MsgID  string          `json:"msg_id"`
	Data   json.RawMessage `json:"payload"`
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[Gateway] WS upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	log.Println("[Gateway] New client connected via WebSocket session!")

	for {
		// Read message from frontend
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[Gateway] Client disconnected or read error: %v", err)
			break
		}

		// Parse the basic structural packet envelope
		var packet ClientPacket
		if err := json.Unmarshal(message, &packet); err != nil {
			log.Printf("[Gateway] Failed to parse client JSON: %v", err)
			continue
		}

		log.Printf("[Gateway] Received client action: '%s', multiplexing to Rust Core...", packet.Action)

		// Inject mock user_id=777 for Phase 1 testing (OAuth integration comes later)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := gameClient.ProcessAction(ctx, &ActionRequest{
			Action:  packet.Action,
			MsgId:   packet.MsgID,
			Payload: string(packet.Data),
			UserId:  777,
		})
		cancel()

		if err != nil {
			log.Printf("[Gateway] gRPC call to Rust Core failed: %v", err)
			continue
		}

		// Forward the clean response payload directly back down the client WebSocket pipeline
		responseJSON := map[string]interface{}{
			"status":  resp.Status,
			"msg_id":  resp.MsgId,
			"payload": resp.Payload,
		}

		finalResponse, _ := json.Marshal(responseJSON)
		_ = conn.WriteMessage(websocket.TextMessage, finalResponse)
	}
}

func main() {
	fmt.Println("[Gateway] Initializing system connection fabrics...")

	// Connect to Rust Core internal container target port using insecure transport for local dev
	grpcConn, err := grpc.Dial("core:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("[Gateway] Could not connect to Rust backend: %v", err)
	}
	defer grpcConn.Close()
	gameClient = NewGameServiceClient(grpcConn)

	// Set up the HTTP routing endpoints
	http.HandleFunc("/ws", handleWebSocket)

	log.Println("[Gateway] Production bridge active. Routing WebSockets on :8080/ws")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("[Gateway] HTTP Server failure: %v", err)
	}
}
