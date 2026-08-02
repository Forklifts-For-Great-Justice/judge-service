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
		// If no DB_DSN is set, proceed without database — useful for
		// local development where the binary only serves /health and /openapi.json.
		log.Println("WARNING: database not configured — /shenanigans routes disabled")
	}

	var repo *repository.ShananiganRepo
	var pub *rabbitmq.Publisher

	if db != nil {
		repo = repository.NewShananiganRepo(db)

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
	handlers.RegisterOpenAPI(reg)

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

	db, err := sql.Open("postgres", dsn)
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

	return db, nil
}
