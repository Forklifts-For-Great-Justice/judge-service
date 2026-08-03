# JudgeService Application Specification

## Overview

JudgeService provides HackFortress judges with a web API for managing the competition: shenanigan catalogue and activation, challenges (puzzles), teams, and rounds.

## Deployment

| Property | Value |
|----------|-------|
| **host** | `judge.hackfortress.net` |
| **container_port** | `8086` |
| **language** | Go |
| **router** | `github.com/go-chi/chi/v5` |
| **database** | PostgreSQL (scoreboard host) |
| **message_bus** | RabbitMQ (topic exchange `hackfortress`) |
| **authn / z** | Handled by nginx / Authelia; service trusts validated headers |

### Authentication

The service does **not** implement its own auth. Every request comes through the nginx proxy which has already validated the JWT against Authelia. The service expects these headers on every request:

| header | source | purpose |
|--------|--------|---------|
| `x-auth-user` | JWT subject | user identity |
| `x-auth-scope` | JWT `scp`/`scope` claim | must include `judge` |
| `x-auth-email` | JWT email claim | optional audit log |

## URL Structure

```
judge.hackfortress.net
├── /health              → no auth ✅
├── /metrics             → no auth ✅
├── /openapi.json        → no auth ✅
├── /shenanigans         → CRUD + activate ✅
├── /challenges          → CRUD  ⬜ (M5)
├── /teams               → CRUD  ✅
└── /rounds              → CRUD  ⬜ (M6)
```

## Endpoints

### Health & Observability

| method | path | auth | description |
|--------|------|------|-------------|
| `GET` | `/health` | *(none)* | Health check. Returns `{"status":"ok"}`. |
| `GET` | `/metrics` | *(none)* | Prometheus metrics (`promhttp.Handler`). |
| `GET` | `/openapi.json` | *(none)* | Serves the OpenAPI 3.0.0 specification as JSON. |

### Shenanigans

**`GET /shenanigans`**

List all catalogue entries.

> `200 OK → { shenanigans: [...] }`

**`POST /shenanigans`**

Create a new catalogue entry.

> Request: `{ name, description, rcon_payload, target_type, cost?, metadata? }`
> `201 Created  → { id: <int>, ... }`

**`GET /shenanigans/{id}`**

Get a single catalogue entry by ID.

> `200 OK → { shenanigan }`

**`PUT /shenanigans/{id}`**

Update an existing catalogue entry.

> Request: `{ name?, description?, rcon_payload?, target_type?, cost?, metadata? }`
> `200 OK → { shenanigan }`

**`DELETE /shenanigans/{id}`**

Delete a catalogue entry.

> `204 No Content`

**`POST /shenanigans/{id}/activate`**

Activate (purchase / trigger) a shenanigan. Resolves the `rcon_payload` from the catalogue entry, publishes a msgpack message to RabbitMQ, and returns the purchase result.

> Request: `{ team: "red", metadata: {} }`
> `200 OK → { purchase_id: "<uuid>", status: "ok", published: true }`

> **Notes:**
> - Judges do **not** deduct HackCoin — this is free for judges.
> - Message is published to the `hackfortress` RabbitMQ exchange with routing key `shenanigans.shenanigan.judge`.
> - `purchase_id` is a UUID used for idempotency by consumers.

### Challenges

**`GET /challenges`**

List all challenges (puzzles).

> `200 OK → { challenges: [...] }`

**`POST /challenges`**

Create a challenge.

> Request: `{ name, description, points, unlock?, category?, quickdraw? }`
> `201 Created → { id, ... }`

**`GET /challenges/{id}`**

Get a single challenge.

> `200 OK → { challenge }`

**`PUT /challenges/{id}`**

Update a challenge.

> Request: `{ name?, description?, points?, unlock?, category?, quickdraw? }`
> `200 OK → { challenge }`

**`DELETE /challenges/{id}`**

Delete a challenge.

> `204 No Content`

### Teams

**`GET /teams`**

List all teams.

> `200 OK → { teams: [...] }`

**`POST /teams`**

Create a team.

> Request: `{ name, color }` (`color` = `"red"` or `"blue"`)
> `201 Created → { id, ... }`

**`GET /teams/{id}`**

Get a single team.

> `200 OK → { team }`

**`PUT /teams/{id}`**

Update a team.

> Request: `{ name?, color? }`
> `200 OK → { team }`

**`DELETE /teams/{id}`**

Delete a team.

> `204 No Content`

### Rounds

**`GET /rounds`**

List all rounds, including game state and automation state.

> `200 OK → { rounds: [...], game: { }, automation: { } }`

**`POST /rounds`**

Create a new round.

> Request: `{ round: { name, puzzleset }, team1, team2 }`
> `201 Created → { id: <int> }`

**`GET /rounds/{id}`**

Get a single round.

> `200 OK → { round }`

**`PUT /rounds/{id}`**

Update a round.

> Request: `{ round: { name?, puzzleset? } }`
> `200 OK → { round }`

**`DELETE /rounds/{id}`**

Delete a round.

> `204 No Content`

**`POST /rounds/{id}/ready`**

Toggle a round to / from **ready** state.

> `200 OK → { }`

**`POST /rounds/{id}/live`**

Toggle a round to / from **live** state. Sets the `live_round_id` on the game table.

> `200 OK → { }`

## Outbound Messages

### RabbitMQ

| routing_key | content-type | when |
|-------------|-------------|------|
| `shenanigans.shenanigan.judge` | `application/vnd.msgpack` | On shenanigan activation |

**Message fields (msgpack-encoded):**

| field | type | required | description |
|-------|------|----------|-------------|
| `purchase_id` | string | yes | UUID — idempotency key |
| `shenanigan_id` | integer | yes | FK to catalogue entry |
| `rcon_payload` | string | yes | RCON command for Quake server |
| `metadata` | object | no | Arbitrary key-value pairs |

Consumers **must** deduplicate on `purchase_id`.

## Environment Variables

| variable | default | description |
|----------|---------|-------------|
| `PORT` | `8086` | HTTP listen port |
| `DB_DSN` | _(required)_ | PostgreSQL connection string |
| `RABBITMQ_URL` | _(required)_ | RabbitMQ AMQP connection string |
| `RABBITMQ_EXCHANGE` | `hackfortress` | Topic exchange name |

## Project Structure

```
JudgeService/
├── cmd/judge/
│   └── main.go              # entrypoint; wires router, DB, RabbitMQ, starts server
├── internal/
│   ├── config/
│   │   └── config.go        # env vars
│   ├── handlers/
│   │   ├── health.go        # /health                        ✅
│   │   ├── shenanigans.go   # /shenanigans/*                 ✅
│   │   ├── teams.go         # /teams/*                       ✅
│   │   ├── challenges.go    # /challenges/*                  ⬜ (Milestone 5)
│   │   └── rounds.go        # /rounds/*                      ⬜ (Milestone 6)
│   ├── models/              # entity structs
│   ├── openapi/
│   │   ├── registry.go      # Route registry, Spec() → map   ✅
│   │   └── schema.go        # SchemaHandler → /openapi.json  ✅
│   ├── repository/
│   │   ├── shenanigan_repo.go # ✅                           ✅
│   │   ├── team_repo.go       # ✅                           ✅
│   │   ├── challenge_repo.go  # ⬜ (Milestone 5)
│   │   └── round_repo.go      # ⬜ (Milestone 6)
│   └── rabbitmq/
│       └── publisher.go     # RabbitMQ publisher (msgpack)   ✅ (M4)
├── openapi/
│   └── schema.json          # static schema (generated)      ✅
├── scripts/
│   └── test_*.py            # external integration tests     ✅
├── go.mod
└── README.md
```

Key: ✅ = implemented, ⬜ = planned (milestone in parentheses)

## Code Patterns

### OpenAPI Spec

The service uses the same dynamic JSON-registry pattern as the PlayerService (`PlayerService/internal/openapi`).

Each handler package registers its routes in `main()` via `openapi.Route` structs:

```go
registry := openapi.NewRegistry()
shenanigan.RegisterRoutes(registry)
challenge.RegisterRoutes(registry)
team.RegisterRoutes(registry)
round.RegisterRoutes(registry)
handler.RegisterHealthRoute(registry)
```

`SchemaHandler(registry)` serves the spec at `GET /openapi.json` as `application/json`. The `/openapi.json` route is also registered so it appears in the output.

The static `openapi/schema.json` file is generated to match the registry output and is used by tests for snapshot verification.

### Repository Layer

Each CRUD resource has its own repository file under `internal/repository/`. Repositories use `github.com/jackc/pgx/v5/pgxpool` for database access. All repositories share a single `*pgxpool.Pool` injected at startup from `DB_DSN`.

### Error Responses

All errors use the same JSON format:

```json
{ "error": "description" }
```

Standard HTTP codes:

| code | when |
|------|------|
| `400` | bad request / invalid parameters |
| `404` | resource not found |
| `500` | internal server error |

### RabbitMQ Publisher

`internal/rabbitmq/publisher.go` implements an AMQP publisher using `streadway/amqp`. It connects to a RabbitMQ server using `RABBITMQ_URL` and publishes msgpack-encoded messages to a topic exchange (`RABBITMQ_EXCHANGE`).

- Connection is auto-reconnecting on failure.
- Publisher is created at startup; if RabbitMQ is unavailable, the service starts with a no-op publisher (returns `published: false` on activate).
- Message encoding: msgpack with fields `purchase_id`, `shenanigan_id`, `rcon_payload`, `metadata`.
- Routing key: `shenanigans.shenanigan.judge`.

```go
publisher := rabbitmq.NewPublisher(config.RabbitMQURL, config.RabbitMQExchange)
// On activate: publisher.Publish(ctx, routingKey, msgpackPayload)
```

## Milestones

### Milestone 1 — Build & Verify ✅ **Complete**

A minimal Go application that proves the project compiles, runs, and is pushed to GitHub in the `Forklifts For Great Justice` org.

**Deliverables:**
- Go module initialized (`go mod init`)
- `cmd/judge/main.go` — `chi` router with two routes:
  - `GET /` — returns `{"message":"Hello, Judge Service!","status":"ok"}`
  - `GET /health` — returns `{"status":"ok"}`
- No database dependency, no RabbitMQ, no auth — just a listening HTTP server
- `Dockerfile` — builds and runs the binary
- `go.mod` / `go.sum` committed and pushed to GitHub

**Verification:**
```bash
go build ./cmd/judge/
./judge    # → listens on :8086
# http://localhost:8086/     → {"message":"Hello, Judge Service!","status":"ok"}
# http://localhost:8086/health → {"status":"ok"}
```

**Goal:** Code compiles, binary runs, repository exists on GH — nothing more. No DB, no MQ, no JWT. Progress from here only after Milestone 1 is on GitHub.

---

### Milestone 2 — Shenanigans CRUD ✅ **Complete**

Full CRUD for the shenanigan catalogue plus the `/activate` endpoint. Includes OpenAPI spec registration, auth middleware, SQLite in-memory repo tests, and DB migration.

**Deliverables:**
- `internal/repository/shenanigan_repo.go` — PostgreSQL persistence (pgx pool)
- `internal/repository/shenanigan_repo_test.go` — SQLite in-memory tests
- `internal/handlers/shenanigans.go` — CRUD + activate handlers with auth middleware
- `internal/handlers/shenanigans_test.go` — handler tests via `httptest`
- `internal/openapi` — route registration for shenanigan endpoints
- `openapi/schema.json` — static OpenAPI snapshot
- DB migration: table creation, `deleted_at` soft-delete on startup
- `POST /shenanigans/{id}/activate` — publishes to RabbitMQ (stubbed in tests)

**Verification:**
```bash
go build ./cmd/judge/
go test ./...
./judge    # listens on :8086
# curl http://localhost:8086/shenanigans   → 200 OK (empty list)
# curl -X POST http://localhost:8086/shenanigans → 201 Created
```

**Goal:** Full shenanigan CRUD pipeline — DB, handlers, repo, tests, OpenAPI — all wired and tested.

---

### Milestone 3 — Teams CRUD ✅ **Complete**

Full CRUD for competition teams.

**Deliverables:**
- `internal/repository/team_repo.go` — PostgreSQL persistence
- `internal/repository/team_repo_test.go` — SQLite in-memory tests
- `internal/handlers/teams.go` — CRUD handlers
- `internal/handlers/teams_test.go` — handler tests
- `internal/models/` — Team struct

**Verification:**
```bash
git push origin main          # triggers CI/CD
# → CI builds Docker image → GHCR
# → Restart judge container on Portainer
# → curl http://localhost:8086/teams → 200 OK
```

**Goal:** Team management is fully functional via API.

---

### Milestone 4 — RabbitMQ Publisher ✅ **Complete and Verified**

RabbitMQ publisher for shenanigan activation messages.

**Deliverables:**
- `internal/rabbitmq/publisher.go` — AMQP publisher using `streadway/amqp`, msgpack encoding
- Publishes to topic exchange `hackfortress` with routing key `shenanigans.shenanigan.judge`
- `POST /shenanigans/{id}/activate` integration: resolves catalogue entry → publishes msgpack payload → returns `purchase_id`
- Connection lifecycle management (auto-reconnect, graceful shutdown)

**Verification:**
```
Verified on production — RabbitMQ publisher confirmed complete and working.
```

**Goal:** Shenanigan activation publishes real messages to RabbitMQ, not a stub. Consumers receive msgpack-encoded messages with `purchase_id`, `shenanigan_id`, `rcon_payload`, and optional `metadata`.

---

### Milestone 5 — Challenges CRUD ⬜ **Next**

Full CRUD for challenges (puzzles) including unlock, category, and quickdraw fields.

**Deliverables:**
- `internal/models/` — Challenge struct
- `internal/repository/challenge_repo.go` — PostgreSQL persistence
- `internal/handlers/challenges.go` — CRUD handlers
- `internal/openapi` — route registration for challenge endpoints
- Updated `openapi/schema.json`

**Expected endpoints:**
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/challenges` | `judge` | List all challenges |
| `POST` | `/challenges` | `judge` | Create a challenge |
| `GET` | `/challenges/{id}` | `judge` | Get a single challenge |
| `PUT` | `/challenges/{id}` | `judge` | Update a challenge |
| `DELETE` | `/challenges/{id}` | `judge` | Delete a challenge |

**Request shapes:**
- Create/Update: `{ name?, description?, points?, unlock?, category?, quickdraw? }`
- Model fields: `id`, `name`, `description`, `points`, `unlock`, `category`, `quickdraw`, `created_at`, `updated_at`, `deleted_at`

**Goal:** Challenges/puzzles are manageable via API.

---

### Milestone 6 — Rounds CRUD + Lifecycle ⬜ **Pending**

Full CRUD for rounds plus state transition endpoints (`ready`, `live`).

**Deliverables:**
- `internal/models/` — Round struct
- `internal/repository/round_repo.go` — PostgreSQL persistence
- `internal/handlers/rounds.go` — CRUD + ready/live handlers
- `internal/openapi` — route registration for round endpoints
- Updated `openapi/schema.json`
- Game table integration: `live_round_id` management

**Expected endpoints:**
| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/rounds` | `judge` | List all rounds + game/automation state |
| `POST` | `/rounds` | `judge` | Create a round |
| `GET` | `/rounds/{id}` | `judge` | Get a single round |
| `PUT` | `/rounds/{id}` | `judge` | Update a round |
| `DELETE` | `/rounds/{id}` | `judge` | Delete a round |
| `POST` | `/rounds/{id}/ready` | `judge` | Toggle ready state |
| `POST` | `/rounds/{id}/live` | `judge` | Toggle live state (sets `live_round_id`) |

**Goal:** Round lifecycle is fully managed — teams can be assigned, rounds set ready and toggled live.

## Scope

### Owns

- **Round lifecycle** — create, update, delete, set ready, set live
- **Team management** — full CRUD
- **Challenge/puzzle definitions** — full CRUD
- **Shenanigan catalogue** — full CRUD
- **Shenanigan activation** — trigger RCON via RabbitMQ

### Does NOT Own

- **Quake game events** — owned by GameService
- **In-game scores / HackCoin balance** — owned by GameService
- **Authentication / authorization** — owned by nginx / Authelia
