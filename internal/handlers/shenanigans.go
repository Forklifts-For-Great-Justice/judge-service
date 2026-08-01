// Package handlers provides HTTP handlers for shenanigans CRUD and activation.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
	"github.com/forklifts-for-great-justice/judge-service/internal/rabbitmq"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

// ShenaniganHandler handles HTTP requests for /shenanigans
type ShenaniganHandler struct {
	repo      repository.Repository
	publisher Publisher
}

// Publisher defines the RabbitMQ publisher interface for shenanigan activation messages.
type Publisher interface {
	Publish(ctx context.Context, msg rabbitmq.ShenaniganMessage) (bool, error)
}

// NewShenaniganHandler creates a new handler with the given repository and optional publisher.
func NewShenaniganHandler(repo repository.Repository, publisher Publisher) *ShenaniganHandler {
	return &ShenaniganHandler{repo: repo, publisher: publisher}
}

// RegisterRoutes wires the shenanigan routes to the chi router.
func RegisterRoutes(r chi.Router, h *ShenaniganHandler) {
	r.Get("/shenanigans", h.HandleList)
	r.Post("/shenanigans", h.HandleCreate)
	r.Get("/shenanigans/{id}", h.HandleGet)
	r.Put("/shenanigans/{id}", h.HandleUpdate)
	r.Delete("/shenanigans/{id}", h.HandleDelete)
	r.Post("/shenanigans/{id}/activate", h.HandleActivate)
}

// RegisterOpenAPI registers the shenanigan routes with the OpenAPI registry.
func RegisterOpenAPI(reg *openapi.Registry) {
	reg.Register(
		openapi.Route{
			Method:      "GET",
			Path:        "/shenanigans",
			OperationID: "listShenanigans",
			Description: "List all shenanigan catalogue entries.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "POST",
			Path:        "/shenanigans",
			OperationID: "createShenanigan",
			Description: "Create a new shenanigan catalogue entry.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 201, Body: "json"},
			},
		},
		openapi.Route{
			Method:      "GET",
			Path:        "/shenanigans/{id}",
			OperationID: "getShenanigan",
			Description: "Get a single shenanigan catalogue entry by ID.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "PUT",
			Path:        "/shenanigans/{id}",
			OperationID: "updateShenanigan",
			Description: "Update an existing shenanigan catalogue entry.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "DELETE",
			Path:        "/shenanigans/{id}",
			OperationID: "deleteShenanigan",
			Description: "Delete a shenanigan catalogue entry.",
			Responses: []openapi.Response{
				{Code: 204, Body: "none", Empty: true, EmptyBody: "No Content"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "POST",
			Path:        "/shenanigans/{id}/activate",
			OperationID: "activateShenanigan",
			Description: "Activate (trigger) a shenanigan. Resolves the rcon_payload, " +
				"publishes a message to RabbitMQ, and returns the purchase result.",
			RequestBody: "json",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
			Notes: "Judges do not deduct HackCoin — this is free for judges. " +
				"Message is published to the exchange with routing key shenanigans.shenanigan.judge.",
		},
	)
}

// HandleList serves GET /shenanigans — list all catalogue entries.
func (h *ShenaniganHandler) HandleList(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	shenanigans, err := h.repo.GetAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list shenanigans: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"shenanigans": shenanigans,
	})
}

// HandleCreate serves POST /shenanigans — create a new catalogue entry.
func (h *ShenaniganHandler) HandleCreate(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	var body struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		RconPayload string          `json:"rcon_payload"`
		TargetType  string          `json:"target_type"`
		Cost        json.Number     `json:"cost,omitempty"`
		Metadata    json.RawMessage `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var cost *int64
	if body.Cost != "" {
		val, err := strconv.ParseInt(body.Cost.String(), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cost must be a valid integer")
			return
		}
		cost = &val
	}

	s := &models.Shananigan{
		Name:        body.Name,
		Description: body.Description,
		RconPayload: body.RconPayload,
		TargetType:  body.TargetType,
		Cost:        cost,
		Metadata:    body.Metadata,
	}

	if err := h.repo.Create(ctx, s); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create shenanigan: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

// HandleGet serves GET /shenanigans/{id} — get a single entry by ID.
func (h *ShenaniganHandler) HandleGet(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	s, err := h.repo.GetByID(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "shenanigan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get shenanigan: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(s)
}

// HandleUpdate serves PUT /shenanigans/{id} — update an existing entry.
func (h *ShenaniganHandler) HandleUpdate(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Name        *string         `json:"name,omitempty"`
		Description *string         `json:"description,omitempty"`
		RconPayload *string         `json:"rcon_payload,omitempty"`
		TargetType  *string         `json:"target_type,omitempty"`
		Cost        *json.Number    `json:"cost,omitempty"`
		Metadata    json.RawMessage `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := make(map[string]any)
	if body.Name != nil {
		updates["name"] = *body.Name
	}
	if body.Description != nil {
		updates["description"] = *body.Description
	}
	if body.RconPayload != nil {
		updates["rcon_payload"] = *body.RconPayload
	}
	if body.TargetType != nil {
		updates["target_type"] = *body.TargetType
	}
	if body.Cost != nil {
		val, err := strconv.ParseInt(body.Cost.String(), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "cost must be a valid integer")
			return
		}
		updates["cost"] = &val
	}
	if body.Metadata != nil {
		updates["metadata"] = body.Metadata
	}

	if len(updates) == 0 {
		writeError(w, http.StatusBadRequest, "at least one field required")
		return
	}

	if err := h.repo.Update(ctx, id, updates); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "shenanigan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update shenanigan: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	updated, err := h.repo.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	json.NewEncoder(w).Encode(updated)
}

// HandleDelete serves DELETE /shenanigans/{id} — delete a catalogue entry.
func (h *ShenaniganHandler) HandleDelete(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.repo.Delete(ctx, id); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "shenanigan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete shenanigan")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleActivate serves POST /shenanigans/{id}/activate — trigger a shenanigan.
func (h *ShenaniganHandler) HandleActivate(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Team     string          `json:"team"`
		Metadata json.RawMessage `json:"metadata,omitempty"`
	}

	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	record, err := h.repo.Activate(ctx, id)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "shenanigan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to activate shenanigan: %v", err))
		return
	}

	published := false
	status := "ok"
	if h.publisher != nil {
		message := rabbitmq.ShenaniganMessage{
			PurchaseID:   record.PurchaseID.String(),
			ShenaniganID: fmt.Sprintf("%d", id),
			RconPayload:  record.RconPayload,
			Metadata:     body.Metadata,
		}

		published, err = h.publisher.Publish(ctx, message)
		if err != nil {
			status = "error"
		}
	}

	resp := map[string]any{
		"purchase_id": record.PurchaseID.String(),
		"status":      status,
		"published":   published,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// writeError encodes a JSON error response and writes it to the response writer.
func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
