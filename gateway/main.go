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

// --- GLOBAL METRICS STRUCT BLUEPRINTS DEFINITIONS ---

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

// Dedicated response block explicitly mapping independent PostgreSQL runtime metrics
type DatabaseTelemetryResponse struct {
	Status        bool   `json:"status"`
	Version       string `json:"version"`
	Uptime        string `json:"uptime"`
	DatabaseState string `json:"database_state"` // Contains active DB string or empty string
}

type TelemetryTreeResponse struct {
	Gateway  ComponentStatus           `json:"gateway"`
	GameCore ComponentStatus           `json:"game_core"`
	Postgres DatabaseTelemetryResponse `json:"postgres"`
	Redis    ComponentStatus           `json:"redis"`
}

type SwitchPayload struct {
	DatabaseName string `json:"database_name"`
}

type MigratePayload struct {
	AdminUsername string `json:"admin_username"`
	AdminPassword string `json:"admin_password"`
}

// --- GLOBAL VARIABLES CONFIGURATION ---

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var gameClient GameServiceClient
var gatewayBootTime time.Time

// --- HELPER UTILITIES ---

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

// --- API HTTP HANDLERS ---

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
	postgresMetric := DatabaseTelemetryResponse{Status: false, Version: "PostgreSQL 16", Uptime: "N/A", DatabaseState: ""}
	redisMetric := ComponentStatus{Status: false, Version: "Redis 7", Uptime: "N/A"}

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	coreResp, err := gameClient.GetServerTelemetry(ctx, &TelemetryRequest{})
	cancel()

	if err == nil {
		coreMetric.Status = coreResp.Status
		coreMetric.Version = coreResp.Version

		// De-multiplexing token schema array: "uptime|pg_server_alive|redis_alive|active_db_label"
		parts := strings.Split(coreResp.Uptime, "|")
		if len(parts) == 4 {
			coreMetric.Uptime = parts[0]
			postgresMetric.Status = (parts[1] == "true")
			redisMetric.Status = (parts[2] == "true")

			postgresMetric.Uptime = parts[0]
			if redisMetric.Status {
				redisMetric.Uptime = parts[0]
			}

			// Dynamic layout filtration parameter mapping: if disconnected, pass an empty string
			activeDB := parts[3]
			if activeDB == "DISCONNECTED" {
				postgresMetric.DatabaseState = ""
			} else {
				postgresMetric.DatabaseState = activeDB
			}
		} else {
			coreMetric.Uptime = coreResp.Uptime
		}
	} else {
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

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/admin/health", handleAdminHealth)
	http.HandleFunc("/api/admin/databases", handleListDatabases)
	http.HandleFunc("/api/admin/databases/switch", handleSwitchDatabase)
	http.HandleFunc("/api/admin/databases/migrate", handleMigrateDatabase)

	log.Println("[Gateway] Production connection bridge running stable on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("[Gateway] Critical socket crash: %v", err)
	}
}
