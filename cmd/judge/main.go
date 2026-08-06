// Package main is the entrypoint for JudgeService.
// It wires the router, database, RabbitMQ publisher, and starts the HTTP server.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	chi "github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/forklifts-for-great-justice/judge-service/internal/handlers"
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
	"github.com/forklifts-for-great-justice/judge-service/internal/rabbitmq"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

func main() {
	r := NewRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	log.Printf("Judge Service starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// NewRouter creates the chi router with all routes wired up.
func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)

	// Prometheus counters — auto-registered with the default registry.
	shenaniganActivationsTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name: "shenanigan_activations_total", Help: "Total activations",
	})
	shenaniganCreationTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name: "shenanigan_creation_total", Help: "Total creations",
	})
	shenaniganPublishFailuresTotal := promauto.NewCounter(prometheus.CounterOpts{
		Name: "shenanigan_publish_failures_total", Help: "Total publish failures",
	})

	db, err := openDB()
	if err != nil {
		log.Printf("WARNING: database connection failed: %v — /shenanigans routes disabled", err)
	}

	var repo repository.Repository
	var teamRepo repository.TeamRepository
	var pub *rabbitmq.Publisher

	if db != nil {
		repo = repository.NewShananiganRepo(db)
		teamRepo = repository.NewTeamRepo(db)

		rabbitMQURL := os.Getenv("RABBITMQ_URL")
		if rabbitMQURL != "" {
			pub, err = rabbitmq.NewPublisher(context.Background(), rabbitMQURL, os.Getenv("RABBITMQ_EXCHANGE"))
			if err != nil {
				log.Printf("WARNING: failed to connect to RabbitMQ — messages will not be published: %v", err)
			}
		}
	}

	// Mount /metrics before handler routes so it works independently of DB/shenanigans.
	r.Handle("/metrics", promhttp.Handler())

	reg := openapi.NewRegistry()

	// Health endpoint
	handlers.RegisterHealthRoute(r, reg)
	handlers.RegisterHealthOpenAPI(reg)

	// Shenanigan routes
	metrics := handlers.NewCounterMetrics(shenaniganActivationsTotal, shenaniganCreationTotal, shenaniganPublishFailuresTotal)
	shenaniganHandler := handlers.NewShenaniganHandler(repo, pub, metrics)
	handlers.RegisterRoutes(r, shenaniganHandler)

	// Team routes
	var teamHandler *handlers.TeamHandler
	if teamRepo != nil {
		teamHandler = handlers.NewTeamHandler(teamRepo)
	}

	// Team routes — READS PUBLIC, MANIPULATIONS AUTHENTICATED
	// Public (no auth)
	if teamHandler != nil {
		r.Get("/teams", teamHandler.HandleList)
		r.Get("/teams/{id}", teamHandler.HandleGet)

		// Authenticated (judge scope)
		r.Group(func(r chi.Router) {
			r.Method("POST", "/teams", handlers.AuthMiddleware(http.HandlerFunc(teamHandler.HandleCreate), "judge"))
			r.Method("PUT", "/teams/{id}", handlers.AuthMiddleware(http.HandlerFunc(teamHandler.HandleUpdate), "judge"))
			r.Method("DELETE", "/teams/{id}", handlers.AuthMiddleware(http.HandlerFunc(teamHandler.HandleDelete), "judge"))
		})
	}

	handlers.RegisterOpenAPI(reg)

	if teamHandler != nil {
		teamHandler.RegisterOpenAPI(reg)
	}

	// Challenge routes — READS PUBLIC, MANIPULATIONS AUTHENTICATED
	var challengeHandler *handlers.ChallengeHandler
	if db != nil {
		challengeRepo := repository.NewChallengeRepo(db)
		challengeHandler = handlers.NewChallengeHandler(challengeRepo)
	}

	if challengeHandler != nil {
		r.Get("/challenges", challengeHandler.HandleList)
		r.Get("/challenges/{id}", challengeHandler.HandleGet)

		r.Group(func(r chi.Router) {
			r.Method("POST", "/challenges", handlers.AuthMiddleware(http.HandlerFunc(challengeHandler.HandleCreate), "judge"))
			r.Method("PUT", "/challenges/{id}", handlers.AuthMiddleware(http.HandlerFunc(challengeHandler.HandleUpdate), "judge"))
			r.Method("DELETE", "/challenges/{id}", handlers.AuthMiddleware(http.HandlerFunc(challengeHandler.HandleDelete), "judge"))
		})

		challengeHandler.RegisterOpenAPI(reg)
	}

	// Round routes — ALL AUTHENTICATED
	var roundHandler *handlers.RoundHandler
	if db != nil {
		roundRepo := repository.NewRoundRepo(db)
		roundHandler = handlers.NewRoundHandler(roundRepo)

		r.Group(func(r chi.Router) {
			r.Method("GET", "/rounds", handlers.AuthMiddleware(http.HandlerFunc(roundHandler.HandleList), "judge"))
			r.Method("POST", "/rounds", handlers.AuthMiddleware(http.HandlerFunc(roundHandler.HandleCreate), "judge"))
			r.Method("GET", "/rounds/{id}", handlers.AuthMiddleware(http.HandlerFunc(roundHandler.HandleGet), "judge"))
			r.Method("PUT", "/rounds/{id}", handlers.AuthMiddleware(http.HandlerFunc(roundHandler.HandleUpdate), "judge"))
			r.Method("DELETE", "/rounds/{id}", handlers.AuthMiddleware(http.HandlerFunc(roundHandler.HandleDelete), "judge"))
			r.Method("POST", "/rounds/{id}/ready", handlers.AuthMiddleware(http.HandlerFunc(roundHandler.HandleToggleReady), "judge"))
			r.Method("POST", "/rounds/{id}/live", handlers.AuthMiddleware(http.HandlerFunc(roundHandler.HandleToggleLive), "judge"))
		})
		roundHandler.RegisterOpenAPI(reg)
	}

	// Scoreboard route — PUBLIC (no auth)
	var sbRepo repository.ScoreboardRepository
	if db != nil {
		sbRepo = repository.NewScoreboardRepo(db)
	}
	sbHandler := handlers.NewScoreboardHandler(sbRepo)
	r.Get("/scoreboard", sbHandler.HandleGet)
	sbHandler.RegisterOpenAPI(reg)

	// Wrap router so SchemaHandler is available on every call.
	// This serves /openapi.json and registers the route in the spec.
	return openapi.SchemaHandlerMiddleware(reg, r)
}

// openDB connects to PostgreSQL using DB_DSN from the environment.
func openDB() (*sql.DB, error) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return nil, nil
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Ensure soft-delete column exists (idempotent migration).
	if _, err := db.Exec("ALTER TABLE shenanigans ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP"); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	// Create teams table (idempotent).
	const teamsTableSQL = `
	CREATE OR REPLACE FUNCTION set_updated_at()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at := NOW();
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;

	CREATE TABLE IF NOT EXISTS team (
		id          SERIAL PRIMARY KEY,
		slug        TEXT NOT NULL UNIQUE,
		name        TEXT NOT NULL,
		alt_name    TEXT NOT NULL UNIQUE,
		clan_tag    TEXT NOT NULL UNIQUE,
		created_at  TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at  TIMESTAMP NOT NULL DEFAULT NOW(),
		CONSTRAINT chk_team_slug_format CHECK (slug ~ '^[a-z0-9-]+$' AND length(slug) BETWEEN 2 AND 64)
	);

	ALTER TABLE team ALTER COLUMN created_at TYPE TIMESTAMP USING created_at::TIMESTAMP;
	ALTER TABLE team ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at::TIMESTAMP;

	DROP TRIGGER IF EXISTS trg_team_updated_at ON team;
	CREATE TRIGGER trg_team_updated_at
		BEFORE UPDATE ON team
		FOR EACH ROW
		EXECUTE FUNCTION set_updated_at();
	`
	if _, err := db.Exec(teamsTableSQL); err != nil {
		return nil, fmt.Errorf("team migration failed: %w", err)
	}

	// Create challenges table (idempotent).
	const challengesTableSQL = `
CREATE TABLE IF NOT EXISTS challenge (
	id         SERIAL PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL,
	challenge_type TEXT,
	location    TEXT,
	points     INTEGER NOT NULL DEFAULT 50,
	disabled   BOOLEAN NOT NULL DEFAULT FALSE,
	flag       TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
	CONSTRAINT chk_challenge_points CHECK (points > 0)
);

ALTER TABLE challenge ALTER COLUMN created_at TYPE TIMESTAMP USING created_at::TIMESTAMP;
ALTER TABLE challenge ALTER COLUMN updated_at TYPE TIMESTAMP USING updated_at::TIMESTAMP;

DROP TRIGGER IF EXISTS trg_challenge_updated_at ON challenge;
CREATE TRIGGER trg_challenge_updated_at
	BEFORE UPDATE ON challenge
	FOR EACH ROW
	EXECUTE FUNCTION set_updated_at();
`
	if _, err := db.Exec(challengesTableSQL); err != nil {
		return nil, fmt.Errorf("challenge migration failed: %w", err)
	}

	// Create matches table if not exists and ensure round management columns exist (idempotent).
	const matchesTableSQL = `
	CREATE TABLE IF NOT EXISTS matches (
		id                  SERIAL PRIMARY KEY,
		team_a_id           INTEGER NOT NULL REFERENCES team(id),
		team_b_id           INTEGER NOT NULL REFERENCES team(id),
		round_name          TEXT NOT NULL,
		team_a_points       INTEGER NOT NULL DEFAULT 0,
		team_b_points       INTEGER NOT NULL DEFAULT 0,
		team_a_hack_points  INTEGER NOT NULL DEFAULT 0,
		team_b_hack_points  INTEGER NOT NULL DEFAULT 0,
		team_a_hackcoins    INTEGER NOT NULL DEFAULT 0,
		team_b_hackcoins    INTEGER NOT NULL DEFAULT 0,
		status              TEXT NOT NULL DEFAULT 'scheduled'
			CONSTRAINT chk_matches_status CHECK (status IN ('scheduled', 'in_progress', 'completed', 'cancelled')),
		disabled            BOOLEAN NOT NULL DEFAULT FALSE,
		ready               BOOLEAN NOT NULL DEFAULT FALSE,
		live                BOOLEAN NOT NULL DEFAULT FALSE,
		ready_at            TIMESTAMPTZ,
		live_at             TIMESTAMPTZ,
		created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		CONSTRAINT chk_matches_different_teams CHECK (team_a_id <> team_b_id)
	);

	ALTER TABLE matches ADD COLUMN IF NOT EXISTS disabled BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE matches ADD COLUMN IF NOT EXISTS ready BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE matches ADD COLUMN IF NOT EXISTS live BOOLEAN NOT NULL DEFAULT FALSE;
	ALTER TABLE matches ADD COLUMN IF NOT EXISTS ready_at TIMESTAMPTZ;
	ALTER TABLE matches ADD COLUMN IF NOT EXISTS live_at TIMESTAMPTZ;

	DROP TRIGGER IF EXISTS trg_matches_updated_at ON matches;
	CREATE TRIGGER trg_matches_updated_at
		BEFORE UPDATE ON matches
		FOR EACH ROW
		EXECUTE FUNCTION set_updated_at();
	`
	if _, err := db.Exec(matchesTableSQL); err != nil {
		return nil, fmt.Errorf("round migration failed: %w", err)
	}

	const currentMatchTableSQL = `
	CREATE TABLE IF NOT EXISTS current_match (
		id                  SERIAL PRIMARY KEY,
		match_id            INTEGER REFERENCES matches(id),
		team_a_id           INTEGER REFERENCES team(id),
		team_b_id           INTEGER REFERENCES team(id),
		round_name          TEXT NOT NULL DEFAULT '',
		team_a_points       INTEGER NOT NULL DEFAULT 0,
		team_b_points       INTEGER NOT NULL DEFAULT 0,
		team_a_hack_points  INTEGER NOT NULL DEFAULT 0,
		team_b_hack_points  INTEGER NOT NULL DEFAULT 0,
		team_a_hackcoins    INTEGER NOT NULL DEFAULT 0,
		team_b_hackcoins    INTEGER NOT NULL DEFAULT 0,
		is_current          BOOLEAN NOT NULL DEFAULT TRUE,
		created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);
	`
	if _, err := db.Exec(currentMatchTableSQL); err != nil {
		return nil, fmt.Errorf("current_match migration failed: %w", err)
	}

	return db, nil
}
