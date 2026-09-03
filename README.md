# OGame Next-Gen: Architecture Specification

This document serves as the Single Source of Truth (SSOT) for the tech stack, component interactions, and core architectural patterns of the game.

## 1. Technology Stack

*   **Target OS:** Linux (Deployment via Docker / Kubernetes).
*   **Frontend:** Vue.js 3 or React (TypeScript) + Vite.
*   **Connection Gateway:** Go (Golang).
*   **Game Core & Database Layer:** Rust (Tokio async runtime + SQLx).
*   **Database:** PostgreSQL (Primary storage) + Redis (Task scheduling & Pub/Sub).

---

## 2. System Components & Roles

### 📂 `frontend/` (Client Application)
*   **Transport:** Communicates exclusively via **a single, persistent WebSocket connection** for the entire app (packet multiplexing).
*   **Time Synchronization:** Client-side countdown timers are synchronized with the server's Unix Timestamp upon connection.
*   **Authentication:** OAuth 2.0 (Google / Facebook social login only) to minimize fake accounts and botting.

### 📂 `gateway/` (Go Connection Manager)
*   **Role:** The "Gatekeeper" and protective shield. It is the only component exposed to the public internet.
*   **Network:** Handles hundreds of thousands of concurrent WebSocket sessions with a minimal RAM footprint.
*   **Security:** Validates OAuth tokens from Google/Facebook, sanitizes packet structures, and mitigates DDoS attacks.
*   **Integration:** Forwards synchronous client requests to the `core/` service via high-speed internal **gRPC (Protobuf)**.

### 📂 `core/` (Rust Game Logic)
*   **Role:** The brain of the game. Isolated within a private subnet with exclusive access to PostgreSQL and Redis.
*   **Logic:** Manages resource production formulas, fleet configuration, and executes the round-based combat simulation engine at bare-metal speeds.
*   **OOP Model:** Implemented using Structs (`struct`), Methods (`impl`), and Traits (`trait`) — prioritizing composition over inheritance.
*   **Web3 Readiness:** Built with Rust (the native language for high-performance networks like Solana/NEAR and premium EVM tooling like `alloy`), ensuring seamless future blockchain integration (NFT fleets, tokenized economies).

---

## 3. Core Architectural Patterns

### A. Resource Generation: Lazy Evaluation
The server **never** updates resource fields every second in the database.
1. When a building upgrade starts, resources are deducted, and an event is pushed to Redis: `{ event_type: "BUILDING_COMPLETE", complete_at: "X" }`.
2. While the timer ticks, the server does nothing.
3. When the player logs in (or a fleet arrives), the `core` calculates the exact accrued resources "on the fly" for the duration of the player's absence, adds them to the balance, and updates the building level in a single microsecond DB operation.

### B. Fleet Missions & Battles: Task Scheduler
1. A player dispatches a fleet -> A task is pushed into a **Redis Sorted Set (ZSET)** using the arrival Unix Timestamp as the score.
2. Background workers (Tokio tasks) in the `core` poll Redis every second for tasks whose execution time has arrived.
3. The worker locks the defender's planet in the database (*Pessimistic Locking*), runs a lazy resource update for that planet, executes the battle math script, saves the battle report, and schedules a return flight.
4. The battle outcome is published via **Redis Pub/Sub** to the `gateway/`, which instantly pushes the notification down the specific player's WebSocket.

---

## 4. WebSocket Packet Format (Protocol)

All bi-directional traffic must strictly follow this JSON or Protobuf payload structure:
```json
{
  "action": "category:action_name",
  "msg_id": "unique_request_uuid_for_sync",
  "payload": { ...data objects... }
}
```
