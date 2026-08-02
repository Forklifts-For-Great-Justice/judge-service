# JudgeService

Go HTTP service for the "shenanigan" API — CRUD on game-activatable abilities (fireballs, grenades, etc.) delivered via RCON.

## Quick commands

```bash
go build ./cmd/judge/          # build
go test ./...                   # all tests
go test ./internal/handlers/   # handler tests only
go test ./internal/repository/ # repo tests (uses SQLite in-memory)
```

## Architecture

- **Router:** chi v5, port 8086
- **Database:** PostgreSQL via `pgx` (DSN from `DB_DSN` env)
- **MQ:** RabbitMQ AMQP for activation publishing (URL from `RABBITMQ_URL`)
- **Auth:** header-based. The service trusts `x-auth-user` and `x-auth-scope` headers injected by nginx/Authelia upstream — it does NOT parse JWTs or validate tokens itself
- **Schema:** OpenAPI 3.0.0 served at `/openapi.json`, dynamically built from router registration (middleware wraps all routes)

## Package layout

| Package | Purpose |
|---|---|
| `cmd/judge/` | Entrypoint — wires DB, MQ, metrics, router |
| `internal/handlers/` | HTTP handlers + auth middleware |
| `internal/repository/` | PostgreSQL persistence layer (interface + impl) |
| `internal/models/` | Domain structs (Shananigan, ActivationRecord) |
| `internal/openapi/` | Dynamic OpenAPI registry + schema middleware |
| `internal/rabbitmq/` | AMQP publisher for activation messages |

## Testing

- External test packages: `package handlers_test` imports `handlers`
- No testify/mocks — hand-written mock structs implementing the same interfaces
- Tests use `httptest.NewRequest` + `chi.NewRouteContext()` + `chi.RouteCtxKey` for simulating URL params
- Repository tests use SQLite in-memory (no PostgreSQL needed)

## Auth details

- Routes under `POST/PUT/DELETE /shenanigans/*` require `judge` scope in `x-auth-scope`
- `x-auth-scope` and `x-auth-groups` are split by space or comma
- `GET/POST /shenanigans/*/activate` and `GET /shenanigans/*/activations` are unauthenticated
- `GET /health` and `GET /metrics` always public

## Gotchas

- Soft-delete: `SoftDelete` sets `deleted_at`; `GetByID` filters `deleted_at IS NULL`. Use `GetByIDDeleted` when you need the record after soft-delete (e.g., the delete handler's reload step)
- DB migration: `ADD COLUMN IF NOT EXISTS deleted_at` runs on startup — no separate migration tool
- No DB? Service starts but all shenanigan routes return 503
- `genschema` binary generates the static OpenAPI schema — built via `go run ./cmd/genschema/`
- Docker image pushed to `ghcr.io/forklifts-for-great-justice/judge-service:main` on push to `main` via GH Actions
