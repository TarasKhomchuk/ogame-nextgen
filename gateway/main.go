// gateway/main.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

//go:generate protoc --proto_path=/proto --go_out=. --go-grpc_out=. /proto/game.proto

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var gameClient GameServiceClient
var gatewayBootTime time.Time

type ClientPacket struct {
	Action string          `json:"action"`
	MsgID  string          `json:"msg_id"`
	Data   json.RawMessage `json:"payload"`
}

type ComponentStatus struct {
	Status  bool   `json:"status"`
	Version string `json:"version"`
	Uptime  string `json:"uptime"`
}

type TelemetryTreeResponse struct {
	Gateway  ComponentStatus `json:"gateway"`
	GameCore ComponentStatus `json:"game_core"`
	Postgres ComponentStatus `json:"postgres"`
	Redis    ComponentStatus `json:"redis"`
}

func formatUptime(bootTime time.Time) string {
	elapsed := time.Since(bootTime)
	days := int(elapsed.Hours()) / 24
	hours := int(elapsed.Hours()) % 24
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
}

// REST HTTP handler orchestra serving the cluster telemetry metrics to Vue 3 admin
// gateway/main.go
// REST HTTP handler orchestra serving the cluster telemetry metrics to Vue 3 admin
func handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	gatewayMetric := ComponentStatus{
		Status:  true,
		Version: "v0.1.0-go",
		Uptime:  formatUptime(gatewayBootTime),
	}

	coreMetric := ComponentStatus{Status: false, Version: "N/A", Uptime: "N/A"}
	postgresMetric := ComponentStatus{Status: false, Version: "PostgreSQL 16", Uptime: "N/A"}
	redisMetric := ComponentStatus{Status: false, Version: "Redis 7", Uptime: "N/A"}

	// Query Game Core microservice via gRPC transport channel
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	coreResp, err := gameClient.GetServerTelemetry(ctx, &TelemetryRequest{})
	cancel()

	if err == nil {
		coreMetric.Status = coreResp.Status
		coreMetric.Version = coreResp.Version

		parts := strings.Split(coreResp.Uptime, "|")
		if len(parts) == 3 {
			coreMetric.Uptime = parts[0]
			postgresMetric.Status = (parts[1] == "true")
			redisMetric.Status = (parts[2] == "true")

			if postgresMetric.Status {
				postgresMetric.Uptime = "Active connected pool"
			}
			if redisMetric.Status {
				redisMetric.Uptime = "Active cache connection"
			}
		} else {
			coreMetric.Uptime = coreResp.Uptime
		}
	} else {
		// CRUCIAL BUG FIX: If gRPC call fails, mark ONLY downstream backend tiers as offline.
		// DO NOT touch gatewayMetric.Status, let it remain TRUE because the gateway is alive!
		log.Printf("[Gateway] Telemetry sync with Core failed: %v", err)
		coreMetric.Status = false
		postgresMetric.Status = false
		redisMetric.Status = false
	}

	telemetryPayload := TelemetryTreeResponse{
		Gateway:  gatewayMetric,
		GameCore: coreMetric,
		Postgres: postgresMetric,
		Redis:    redisMetric,
	}

	json.NewEncoder(w).Encode(telemetryPayload)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var packet ClientPacket
		if err := json.Unmarshal(message, &packet); err != nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := gameClient.ProcessAction(ctx, &ActionRequest{
			Action:  packet.Action,
			MsgId:   packet.MsgID,
			Payload: string(packet.Data),
			UserId:  777,
		})
		cancel()

		if err != nil {
			continue
		}

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
	gatewayBootTime = time.Now()
	fmt.Println("[Gateway] Cluster fabrics active. Initializing proxies...")

	grpcConn, err := grpc.Dial("core:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("[Gateway] Connection target core lost: %v", err)
	}
	defer grpcConn.Close()
	gameClient = NewGameServiceClient(grpcConn)

	// Routing mappings
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/admin/health", func(w http.ResponseWriter, r *http.Request) {
		// Adapt proxy wrapper context redirection signatures
		handleAdminHealth(w, nil)
	})

	log.Println("[Gateway] Systems locked. Polling bridge operational on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("[Gateway] Critical socket crash: %v", err)
	}
}
