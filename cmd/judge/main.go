package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"message":"Hello, Judge Service!","status":"ok"}`)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok"}`)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8086"
	}

	log.Printf("Judge Service starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
