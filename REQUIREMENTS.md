# OGame Next-Gen: Component Requirements & Phase 1 Specifications

This document outlines the local environment requirements and the technical specifications for Phase 1 (The Interaction Skeleton) using a Docker-centric development workflow.

## 1. Local Environment Requirements (Docker-Only Setup)

To develop all components without installing language runtimes directly on Windows, only the following tool is required on the host machine:

*   **Host OS Tool:**
    *   **Docker Desktop for Windows:** Must be installed and running with WSL2 backend enabled.
*   **Containerized Environments (Managed via Docker Compose):**
    *   **Frontend Container:** Uses `node:20-alpine` image.
    *   **Gateway Container:** Uses `golang:1.22-alpine` image.
    *   **Game Core Container:** Uses `rust:1.75-alpine` image (with `protoc` installed inside the container via `apk`).

All source code folders from Windows (`frontend/`, `gateway/`, `core/`) are mounted into their respective containers via Docker Volumes. This enables hot-reloading and automatic compilation inside Linux containers upon saving files in Windows.

---

## 2. Phase 1: Minimum Viable Interaction (MVI) Specifications

The goal of Phase 1 is to build three minimalist applications that establish a complete end-to-end communication loop without databases or business logic.

### A. Frontend Component (`frontend/`)
*   **Tech Stack:** React or Vue 3 + Vite (TypeScript).
*   **Requirements:**
    *   Initiate a single WebSocket connection to the Go Gateway on startup (`ws://localhost:8080/ws`).
    *   Render a simple UI with a single button: `"Trigger Action"`.
    *   When the button is clicked, send a JSON packet over WebSocket: `{"action": "ping", "msg_id": "123"}`.
    *   Listen for incoming WebSocket messages and print the server's response on the screen.

### B. Gateway Component (`gateway/`)
*   **Tech Stack:** Go (Golang).
*   **Requirements:**
    *   Expose a public WebSocket endpoint inside the container, mapped to `ws://localhost:8080/ws` on the host.
    *   Accept incoming client connections.
    *   Forward incoming WebSocket packets synchronously to the Rust Game Core using **gRPC** via internal Docker networking (`core:50051`).
    *   Receive the gRPC response from Rust, wrap it back into a WebSocket frame, and send it to the client.

### C. Game Core Component (`core/`)
*   **Tech Stack:** Rust (Tokio async runtime + `tonic` gRPC framework).
*   **Requirements:**
    *   Expose an internal gRPC server endpoint at `0.0.0.0:50051` (accessible via internal Docker network).
    *   Implement a simple RPC method (`ProcessAction`).
    *   When invoked by the Go Gateway, print the received action to the console and return a mock success message: `{"status": "processed", "reply": "pong"}`.

---

## 3. Future Roadmap Phases (For Reference)

*   **Phase 2:** Connect Database (PostgreSQL + Redis) as additional containers in `docker-compose.yml`.
*   **Phase 3:** Build an independent Administration Frontend container.
*   **Phase 4:** Implement Remote Database Script Execution initiated strictly from the Administration Frontend.

### Modified Requirements for Administration Portal (admin-portal/)
*   **Status Panel Feature:**
    *   Render a live dashboard showing the real-time operational status of all services: `admin-portal`, `gateway`, `core`, `postgres`, and `redis`.
    *   On component mount, initiate a REST HTTP fetch request to the Go Gateway polling endpoint (e.g., `/api/admin/health`) every 5 seconds.
    *   Dynamically color-code statuses: Green (`ONLINE`) or Red (`OFFLINE`).

### Modified Requirements for Gateway Component (gateway/)
*   **Health Check Orchestrator Endpoint:**
    *   Expose a secure public HTTP endpoint `GET /api/admin/health` explicitly for the Administration Portal.
    *   Upon request, perform sub-system checks: ping internal Rust Core gRPC endpoint, verify downstream DB connections, and return a structured JSON status tree.

### Modified Specifications for Health Orchestration
*   **Extended Telemetry Tree Schema:**
    *   The `GET /api/admin/health` endpoint on Go Gateway must collect and return an extended metrics payload for each operational tier:
        *   `status`: boolean (true = `ONLINE`, false = `OFFLINE`)
        *   `version`: string (semantic versioning tag, e.g., `v0.1.0`)
        *   `uptime`: string (human-readable service run duration, e.g., `0d 2h 15m 30s`)
