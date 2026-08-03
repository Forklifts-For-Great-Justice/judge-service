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

## CI/CD Pipeline

1. Push to `main` → GH Actions triggers "Build and push Docker image" workflow
2. Workflow builds the Docker image from the Dockerfile and pushes to GHCR
3. Restart the `judge` container on Portainer (env 6, scoreboard environment) to pull the new image
4. Wait for health check — the container exposes `http://localhost:8086/health` and the health check runs `wget -qO- http://localhost:8086/health` every 30s
5. Verify with `curl http://judge.hackfortress.net:8086/health` — should return `{"status":"ok"}`
6. Re-run `python3 scripts/test_teams.py full-cycle` to validate the change end-to-end

### Deploy Checklist

- [ ] All `go test ./...` pass locally
- [ ] `git push origin main`
- [ ] `gh run list --repo Forklifts-For-Great-Justice/judge-service --limit 1` shows `in_progress` → wait → `completed` / `success`
- [ ] Restart container on Portainer env 6 (`judge`)
- [ ] Wait for health check (`starting` → `healthy`)
- [ ] Confirm via `curl http://judge.hackfortress.net:8086/health`
- [ ] Run integration test: `python3 scripts/test_teams.py full-cycle`

### Never copy binaries to the server

- **Never** run `docker cp`, `ssh`, `scp`, Salt `cmd.run`, or any ad-hoc mechanism to place a binary file into a running container or host.
- All server updates must go through the CI/CD pipeline: commit → push → GH Actions build → GHCR push → Portainer container restart.
- A container restart alone does NOT pull a new image if the local tag is out of date. After a CI build completes, you must pull the updated image (via Portainer stack update / `docker-compose pull && docker-compose up -d`) **or** rely on `unless-stopped` restart not being sufficient — in this project, the image tag is `main` which is immutable per commit, so restarts pick up the latest pushed image on the next `docker run` cycle.
- If you need to deploy a fix immediately and the pipeline is failing, report the failure — do not bypass the pipeline.
