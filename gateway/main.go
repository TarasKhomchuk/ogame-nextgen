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

type SwitchPayload struct {
	DatabaseName string `json:"database_name"`
}

type MigratePayload struct {
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
}

func formatUptime(bootTime time.Time) string {
	elapsed := time.Since(bootTime)
	days := int(elapsed.Hours()) / 24
	hours := int(elapsed.Hours()) % 24
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, seconds)
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	gatewayMetric := ComponentStatus{
		Status:  true,
		Version: "v0.1.0-go",
		Uptime:  formatUptime(gatewayBootTime),
	}

	coreMetric := ComponentStatus{Status: false, Version: "N/A", Uptime: "N/A"}
	postgresMetric := ComponentStatus{Status: false, Version: "PostgreSQL 16", Uptime: "N/A"}
	redisMetric := ComponentStatus{Status: false, Version: "Redis 7", Uptime: "N/A"}

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	coreResp, err := gameClient.GetServerTelemetry(ctx, &TelemetryRequest{})
	cancel()

	if err == nil {
		coreMetric.Status = coreResp.Status
		coreMetric.Version = coreResp.Version

		// De-multiplexing token schema: "uptime|pg_system_alive|redis_alive|active_db_label"
		parts := strings.Split(coreResp.Uptime, "|")
		if len(parts) == 4 {
			coreMetric.Uptime = parts[0]

			// 1. Evaluate pure engine operational states (Are containers running and reachable?)
			postgresMetric.Status = (parts[1] == "true")
			redisMetric.Status = (parts[2] == "true")

			// 2. Map contextual logic metadata safely without toggling infrastructural state indicators
			activeDB := parts[3]
			postgresMetric.Version = activeDB // This holds the database label (e.g., 'DISCONNECTED' or 'ogame_game_db')

			if redisMetric.Status {
				redisMetric.Uptime = "Active cache bridge"
			}
		} else {
			coreMetric.Uptime = coreResp.Uptime
		}
	} else {
		log.Printf("[Gateway] Telemetry sync with Core failed: %v", err)
		coreMetric.Status = false
		postgresMetric.Status = false
		redisMetric.Status = false
	}

	json.NewEncoder(w).Encode(TelemetryTreeResponse{
		Gateway:  gatewayMetric,
		GameCore: coreMetric,
		Postgres: postgresMetric,
		Redis:    redisMetric,
	})
}

// REST HTTP Proxy 1: Fetch list of all databases in PostgreSQL catalog instance
func handleListDatabases(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	resp, err := gameClient.GetDatabaseCatalog(ctx, &DatabaseCatalogRequest{})
	cancel()

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query database catalog: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(resp.Databases)
}

// REST HTTP Proxy 2: Command Rust Core to programmatically CREATE or HOT-SWAP an active database
func handleSwitchDatabase(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var payload SwitchPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload context", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	resp, err := gameClient.SwitchOrCreateDatabase(ctx, &SwitchDatabaseRequest{
		DatabaseName: payload.DatabaseName,
	})
	cancel()

	if err != nil {
		http.Error(w, fmt.Sprintf("gRPC database hot-swap failure: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": resp.Success,
		"message": resp.Message,
	})
}

// REST HTTP Proxy 3: Command Rust Core to read .sql blueprint and build tables structure
func handleMigrateDatabase(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var payload MigratePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload structure", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := gameClient.InitDatabaseSchema(ctx, &InitSchemaRequest{
		AdminUsername: payload.AdminUsername,
		AdminPassword: payload.AdminPassword,
	})
	cancel()

	if err != nil {
		http.Error(w, fmt.Sprintf("gRPC DDL transmission migration failure: %v", err), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": resp.Success,
		"message": resp.Message,
	})
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
	fmt.Println("[Gateway] Dynamic API Routing cluster initializing...")

	grpcConn, err := grpc.Dial("core:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("[Gateway] Connection target core lost: %v", err)
	}
	defer grpcConn.Close()
	gameClient = NewGameServiceClient(grpcConn)

	// Traditional endpoints routings
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/admin/health", handleAdminHealth)

	// New Dynamic Database Catalog Administration endpoints API mappings
	http.HandleFunc("/api/admin/databases", handleListDatabases)
	http.HandleFunc("/api/admin/databases/switch", handleSwitchDatabase)
	http.HandleFunc("/api/admin/databases/migrate", handleMigrateDatabase)

	log.Println("[Gateway] Production connection bridge running stable on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("[Gateway] Critical socket crash: %v", err)
	}
}
