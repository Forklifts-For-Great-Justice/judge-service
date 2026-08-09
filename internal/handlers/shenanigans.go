// Package handlers provides HTTP handlers for shenanigans CRUD and activation.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/forklifts-for-great-justice/judge-service/internal/models"
	"github.com/forklifts-for-great-justice/judge-service/internal/openapi"
	"github.com/forklifts-for-great-justice/judge-service/internal/rabbitmq"
	"github.com/forklifts-for-great-justice/judge-service/internal/repository"
)

// ShenaniganHandler handles HTTP requests for /shenanigans
type ShenaniganHandler struct {
	repo          repository.Repository
	publisher     Publisher
	publisherAtomic atomic.Value
	metrics       ShenaniganMetrics
}

// Publisher defines the RabbitMQ publisher interface for shenanigan activation messages.
type Publisher interface {
	Publish(ctx context.Context, msg rabbitmq.ShenaniganMessage) (bool, error)
}

// ShenaniganMetrics defines the Prometheus counter interface for shenanigan metrics.
type ShenaniganMetrics interface {
	IncrementActivations()
	IncrementCreations()
	IncrementPublishFailures()
}

// counterMetrics implements ShenaniganMetrics using three Prometheus counters.
type counterMetrics struct {
	activations     prometheus.Counter
	creations       prometheus.Counter
	publishFailures prometheus.Counter
}

// NewCounterMetrics creates a ShenaniganMetrics from three Prometheus counters.
func NewCounterMetrics(activations, creations, publishFailures prometheus.Counter) ShenaniganMetrics {
	return &counterMetrics{
		activations:     activations,
		creations:       creations,
		publishFailures: publishFailures,
	}
}

func (c *counterMetrics) IncrementActivations() { c.activations.Inc() }
func (c *counterMetrics) IncrementCreations()   { c.creations.Inc() }
func (c *counterMetrics) IncrementPublishFailures() { c.publishFailures.Inc() }

// NewShenaniganHandler creates a new handler with the given repository, publisher, and metrics.
func NewShenaniganHandler(repo repository.Repository, publisher Publisher, metrics ShenaniganMetrics) *ShenaniganHandler {
	h := &ShenaniganHandler{repo: repo, publisher: publisher, metrics: metrics}
	if publisher != nil {
		h.publisherAtomic.Store(publisher)
	}
	return h
}

// SetPublisher dynamically sets or updates the publisher.
func (h *ShenaniganHandler) SetPublisher(publisher Publisher) {
	if publisher != nil {
		h.publisherAtomic.Store(publisher)
	}
}

// RegisterRoutes wires the shenanigan routes to the chi router.
// Admin and catalogue/activation routes require judge scope auth.
func RegisterRoutes(r chi.Router, h *ShenaniganHandler) {
	r.Method("GET", "/shenanigans", AuthMiddleware(http.HandlerFunc(h.HandleList), "judge"))
	r.Method("POST", "/shenanigans", AuthMiddleware(http.HandlerFunc(h.HandleCreate), "judge"))
	r.Method("GET", "/shenanigans/{id}", AuthMiddleware(http.HandlerFunc(h.HandleGet), "judge"))

	r.Method("PUT", "/shenanigans/{id}", AuthMiddleware(http.HandlerFunc(h.HandleUpdate), "judge"))
	r.Method("DELETE", "/shenanigans/{id}", AuthMiddleware(http.HandlerFunc(h.HandleDelete), "judge"))

	r.Method("POST", "/shenanigans/{id}/activate", AuthMiddleware(http.HandlerFunc(h.HandleActivate), "judge"))
	r.Method("GET", "/shenanigans/{id}/activations", AuthMiddleware(http.HandlerFunc(h.HandleListActivations), "judge"))
	r.Method("GET", "/activations/{purchase_id}", AuthMiddleware(http.HandlerFunc(h.HandleGetActivation), "judge"))
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
			Description: "Soft-delete a shenanigan catalogue entry. Returns the updated entry with deleted_at set.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
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
		openapi.Route{
			Method:      "GET",
			Path:        "/shenanigans/{id}/activations",
			OperationID: "listActivations",
			Description: "List all activation records for a shenanigan, optionally filtered by status query param.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
		openapi.Route{
			Method:      "GET",
			Path:        "/activations/{purchase_id}",
			OperationID: "getActivation",
			Description: "Retrieve a single activation record by purchase ID. Used for idempotency checks during activation confirmation.",
			Responses: []openapi.Response{
				{Code: 200, Body: "json"},
				{Code: 404, Body: "json", Empty: false, EmptyBody: "Not Found"},
			},
		},
	)
}

// HandleList serves GET /shenanigans — list all catalogue entries.
func (h *ShenaniganHandler) HandleList(w http.ResponseWriter, req *http.Request) {
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()

	q := req.URL.Query()

	var minCost, maxCost *int64
	if v := q.Get("min_cost"); v != "" {
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid min_cost")
			return
		}
		minCost = &val
	}
	if v := q.Get("max_cost"); v != "" {
		val, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid max_cost")
			return
		}
		maxCost = &val
	}

	targetType := q.Get("target_type")

	page := 1
	if v := q.Get("page"); v != "" {
		p, err := strconv.Atoi(v)
		if err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 50
	if v := q.Get("page_size"); v != "" {
		ps, err := strconv.Atoi(v)
		if err == nil && ps > 0 {
			pageSize = ps
		}
	}
	const maxPageSize = 200
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	opts := &repository.FilterOptions{
		TargetType: targetType,
		MinCost:    minCost,
		MaxCost:    maxCost,
		Page:       page,
		PageSize:   pageSize,
	}

	shenanigans, total, err := h.repo.GetFiltered(ctx, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list shenanigans: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Total-Count", strconv.FormatInt(total, 10))
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Page-Size", strconv.Itoa(pageSize))
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"shenanigans": shenanigans,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
	})
}

// HandleCreate serves POST /shenanigans — create a new catalogue entry.
func (h *ShenaniganHandler) HandleCreate(w http.ResponseWriter, req *http.Request) {
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

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

	if err := s.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.Create(ctx, s); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create shenanigan: %v", err))
		return
	}

	if h.metrics != nil {
		h.metrics.IncrementCreations()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(s)
}

// HandleGet serves GET /shenanigans/{id} — get a single entry by ID.
func (h *ShenaniganHandler) HandleGet(w http.ResponseWriter, req *http.Request) {
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

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
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

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
		updates["name"] = string(*body.Name)
	}
	if body.Description != nil {
		updates["description"] = string(*body.Description)
	}
	if body.RconPayload != nil {
		updates["rcon_payload"] = string(*body.RconPayload)
	}
	if body.TargetType != nil {
		updates["target_type"] = string(*body.TargetType)
	}
	if body.Cost != nil {
		val, err := body.Cost.Int64()
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

	// Validate target_type if it is being changed
	if tt, ok := updates["target_type"].(string); ok {
		if tt != "team" && tt != "all" {
			writeError(w, http.StatusBadRequest, "target_type must be \"team\" or \"all\"")
			return
		}
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

// HandleDelete serves DELETE /shenanigans/{id} — soft delete a catalogue entry.
func (h *ShenaniganHandler) HandleDelete(w http.ResponseWriter, req *http.Request) {
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	if err := h.repo.SoftDelete(ctx, id); err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "shenanigan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete shenanigan")
		return
	}

	// Return the soft-deleted record with deleted_at set.
	s, err := h.repo.GetByIDDeleted(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to reload shenanigan: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(s)
}

// HandleActivate serves POST /shenanigans/{id}/activate — trigger a shenanigan.
func (h *ShenaniganHandler) HandleActivate(w http.ResponseWriter, req *http.Request) {
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

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
	if h.metrics != nil {
		h.metrics.IncrementActivations()
	}
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "shenanigan not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to activate shenanigan: %v", err))
		return
	}

	actRecord := &models.ActivationRecord{
		PurchaseID:   record.PurchaseID,
		ShenaniganID: id,
		Status:       "confirmed",
		RconPayload:  record.RconPayload,
		Metadata:     body.Metadata,
	}
	_ = h.repo.SaveActivation(ctx, actRecord)

	published := false
	status := "ok"
	pub := h.publisher
	if v := h.publisherAtomic.Load(); v != nil {
		if p, ok := v.(Publisher); ok {
			pub = p
		}
	}

	if pub != nil {
		message := rabbitmq.ShenaniganMessage{
			PurchaseID:   record.PurchaseID.String(),
			ShenaniganID: id,
			RconPayload:  record.RconPayload,
			Metadata:     body.Metadata,
		}

		published, err = pub.Publish(ctx, message)
		if err != nil {
			if h.metrics != nil {
				h.metrics.IncrementPublishFailures()
			}
			status = "error"
		}
	}

	resp := map[string]any{
		"purchase_id": record.PurchaseID.String(),
		"status":      status,
		"published":   published,
	}
	if err != nil {
		resp["error"] = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// HandleListActivations serves GET /shenanigans/{id}/activations — list activations for a shenanigan.
// Optional query param "status" filters by activation status.
func (h *ShenaniganHandler) HandleListActivations(w http.ResponseWriter, req *http.Request) {
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	// Verify the shenanigan exists (even soft-deleted ones are visible via
	// the regular GetByID which does not exist, since the shenanigan may have
	// been soft-deleted — we just want to know if an ID was provided).
	// Use GetShenaniganByID which calls GetByID (non-deleted only).
	if _, err := h.repo.GetByIDDeleted(ctx, id); err != nil {
		writeError(w, http.StatusNotFound, "shenanigan not found")
		return
	}

	statusFilter := req.URL.Query().Get("status")
	activations, err := h.repo.GetActivationsForShenanigan(ctx, id, statusFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list activations: %v", err))
		return
	}

	if activations == nil {
		activations = []*models.ActivationRecord{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"activations": activations,
	})
}

// HandleGetActivation serves GET /activations/{purchase_id} — retrieve a single activation by purchase ID.
func (h *ShenaniganHandler) HandleGetActivation(w http.ResponseWriter, req *http.Request) {
	if isNilRepo(h.repo) {
		writeError(w, http.StatusServiceUnavailable, "database not available")
		return
	}

	ctx := req.Context()
	purchaseID, err := uuid.Parse(chi.URLParam(req, "purchase_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid purchase_id")
		return
	}

	activation, err := h.repo.GetActivationByPurchaseID(ctx, purchaseID)
	if err != nil {
		if err == repository.ErrNotFound {
			writeError(w, http.StatusNotFound, "activation not found")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get activation: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(activation)
}

// AuthMiddleware middleware that enforces required scope (e.g. judge).
// Returns 401 if x-auth-user is missing, 403 if expected scope is absent.
func AuthMiddleware(next http.Handler, expectedScope string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("x-auth-user")
		if user == "" {
			writeError(w, http.StatusUnauthorized, "missing x-auth-user header")
			return
		}

		scopes := r.Header.Get("x-auth-scope")
		if scopes == "" {
			scopes = r.Header.Get("x-auth-groups")
		}
		if !contains(scopes, expectedScope) {
			writeError(w, http.StatusForbidden, "missing required scope: "+expectedScope)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// contains checks whether the space/comma-separated scopes or groups string contains the target scope.
func contains(list, target string) bool {
	fields := strings.FieldsFunc(list, func(r rune) bool {
		return r == ',' || r == ' '
	})
	for _, g := range fields {
		if strings.TrimSpace(g) == target {
			return true
		}
	}
	return false
}

// isNilRepo returns true if the repository interface is nil or contains a nil concrete pointer or nil db field.
func isNilRepo(r repository.Repository) bool {
	if r == nil {
		return true
	}
	v := reflect.ValueOf(r)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return true
		}
		if elem := v.Elem(); elem.Kind() == reflect.Struct {
			dbField := elem.FieldByName("db")
			if dbField.IsValid() && dbField.Kind() == reflect.Ptr && dbField.IsNil() {
				return true
			}
		}
	}
	return false
}

// writeError encodes a JSON error response and writes it to the response writer.
func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
