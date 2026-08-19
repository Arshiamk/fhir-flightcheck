package controlplane

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const problemBase = "https://flightcheck.dev/problems/"

var safeTraceID = regexp.MustCompile(`^[A-Za-z0-9._:-]{3,128}$`)

type API struct {
	service *Service
	logger  *slog.Logger
	mux     *http.ServeMux
	auth    AuthConfig
}

type AuthConfig struct {
	RequireAPIAuth bool
	APIToken       string
	WorkerToken    string
}

func NewHandler(service *Service, logger *slog.Logger) http.Handler {
	return NewHandlerWithAuth(service, logger, AuthConfig{})
}

func NewHandlerWithAuth(service *Service, logger *slog.Logger, auth AuthConfig) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	api := &API{service: service, logger: logger, mux: http.NewServeMux(), auth: auth}
	api.routes()
	return api.middleware(api.mux)
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /healthz", a.health)
	a.mux.HandleFunc("GET /readyz", a.ready)
	a.mux.HandleFunc("GET /v1/signing-key", a.signingKey)
	a.mux.HandleFunc("POST /v1/projects", a.createProject)
	a.mux.HandleFunc("GET /v1/projects/{projectID}", a.getProject)
	a.mux.HandleFunc("POST /v1/projects/{projectID}/targets", a.createTarget)
	a.mux.HandleFunc("GET /v1/projects/{projectID}/targets/{targetID}", a.getTarget)
	a.mux.HandleFunc("POST /v1/projects/{projectID}/runs", a.createRun)
	a.mux.HandleFunc("PUT /v1/projects/{projectID}/baseline", a.setBaseline)
	a.mux.HandleFunc("GET /v1/projects/{projectID}/baseline", a.getBaseline)
	a.mux.HandleFunc("GET /v1/runs/{runID}", a.getRun)
	a.mux.HandleFunc("GET /v1/runs/{runID}/report", a.getReport)
	a.mux.HandleFunc("POST /internal/v1/jobs/{jobID}/complete", a.completeJob)
	a.mux.HandleFunc("/", a.notFound)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	if a.service == nil || a.service.Repository == nil || a.service.Signer == nil {
		a.problem(w, r, http.StatusServiceUnavailable, "NOT_READY", "Service unavailable", "Required service dependencies are unavailable.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := a.service.Repository.Ping(ctx); err != nil {
		a.problem(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service unavailable", "A required durable dependency is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (a *API) signingKey(w http.ResponseWriter, r *http.Request) {
	if a.service.Signer == nil {
		a.problem(w, r, http.StatusServiceUnavailable, "SIGNING_UNAVAILABLE", "Signing unavailable", "The report signing key is unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"algorithm": "Ed25519", "keyId": a.service.Signer.KeyID(),
		"publicKey": a.service.Signer.PublicKeyBase64(),
	})
}

func (a *API) notFound(w http.ResponseWriter, r *http.Request) {
	a.problem(w, r, http.StatusNotFound, "ENDPOINT_NOT_FOUND", "Endpoint not found", "The requested API endpoint does not exist.")
}

func (a *API) completeJob(w http.ResponseWriter, r *http.Request) {
	var input CompletionInput
	if !a.decode(w, r, &input) {
		return
	}
	pathJobID := r.PathValue("jobID")
	if input.JobID != "" && input.JobID != pathJobID {
		a.problem(w, r, http.StatusConflict, "STALE_COMPLETION", "Stale completion", "The completion job ID does not match the requested job.")
		return
	}
	input.JobID = pathJobID
	report, replayed, err := a.service.CompleteJob(r.Context(), input)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, createdStatus(replayed), report)
}

func (a *API) createProject(w http.ResponseWriter, r *http.Request) {
	key, ok := a.idempotencyKey(w, r)
	if !ok {
		return
	}
	var input struct {
		Name string `json:"name"`
	}
	if !a.decode(w, r, &input) {
		return
	}
	value, replayed, err := a.service.CreateProject(r.Context(), input.Name, key)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, createdStatus(replayed), value)
}

func (a *API) getProject(w http.ResponseWriter, r *http.Request) {
	value, err := a.service.GetProject(r.Context(), r.PathValue("projectID"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *API) createTarget(w http.ResponseWriter, r *http.Request) {
	key, ok := a.idempotencyKey(w, r)
	if !ok {
		return
	}
	var input CreateTargetInput
	if !a.decode(w, r, &input) {
		return
	}
	value, replayed, err := a.service.CreateTarget(r.Context(), r.PathValue("projectID"), input, key)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, createdStatus(replayed), value)
}

func (a *API) getTarget(w http.ResponseWriter, r *http.Request) {
	value, err := a.service.GetTarget(r.Context(), r.PathValue("projectID"), r.PathValue("targetID"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *API) createRun(w http.ResponseWriter, r *http.Request) {
	key, ok := a.idempotencyKey(w, r)
	if !ok {
		return
	}
	var input CreateRunInput
	if !a.decode(w, r, &input) {
		return
	}
	value, replayed, err := a.service.CreateRun(r.Context(), r.PathValue("projectID"), input, key)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, createdStatus(replayed), value)
}

func (a *API) getRun(w http.ResponseWriter, r *http.Request) {
	value, err := a.service.GetRun(r.Context(), r.PathValue("runID"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *API) getReport(w http.ResponseWriter, r *http.Request) {
	value, err := a.service.GetReport(r.Context(), r.PathValue("runID"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *API) setBaseline(w http.ResponseWriter, r *http.Request) {
	key, ok := a.idempotencyKey(w, r)
	if !ok {
		return
	}
	var input struct {
		RunID string `json:"runId"`
	}
	if !a.decode(w, r, &input) {
		return
	}
	value, replayed, err := a.service.SetBaseline(r.Context(), r.PathValue("projectID"), input.RunID, key)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, createdStatus(replayed), value)
}

func (a *API) getBaseline(w http.ResponseWriter, r *http.Request) {
	value, err := a.service.GetBaseline(r.Context(), r.PathValue("projectID"))
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *API) idempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) < 8 || len(key) > 128 || strings.ContainsAny(key, "\r\n") {
		a.problem(w, r, http.StatusBadRequest, "INVALID_IDEMPOTENCY_KEY", "Invalid idempotency key", "Idempotency-Key must contain 8 to 128 characters.")
		return "", false
	}
	return key, true
}

func (a *API) decode(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		a.problem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request", "The request body is not valid for this endpoint.")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		a.problem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid request", "The request body must contain one JSON value.")
		return false
	}
	return true
}

func (a *API) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var validation *ValidationError
	switch {
	case errors.Is(err, ErrNotFound):
		a.problem(w, r, http.StatusNotFound, "RESOURCE_NOT_FOUND", "Resource not found", "The requested resource does not exist.")
	case errors.Is(err, ErrIdempotencyConflict):
		a.problem(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "Idempotency conflict", "The idempotency key was already used for a different request.")
	case errors.Is(err, ErrStaleCompletion):
		a.problem(w, r, http.StatusConflict, "STALE_COMPLETION", "Stale completion", "The completion does not match an active job.")
	case errors.Is(err, ErrCancelledRun):
		a.problem(w, r, http.StatusConflict, "RUN_CANCELLED", "Run cancelled", "Cancelled runs cannot accept completion results.")
	case errors.As(err, &validation):
		a.problem(w, r, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request", Redact(validation.Error()))
	case errors.Is(err, context.DeadlineExceeded):
		a.problem(w, r, http.StatusGatewayTimeout, "REQUEST_TIMEOUT", "Request timeout", "The request deadline was exceeded.")
	default:
		a.logger.Error("request failed", "error", Redact(err.Error()), "trace_id", traceID(r.Context()))
		a.problem(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error", "The service could not complete the request.")
	}
}

func (a *API) problem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	writeJSONStatus(w, status, "application/problem+json", Problem{
		SchemaVersion: SchemaVersion,
		Type:          problemBase + strings.ToLower(strings.ReplaceAll(code, "_", "-")),
		Title:         title, Status: status, Detail: detail, Instance: r.URL.Path,
		Category: problemCategory(status, code), Code: code, TraceID: traceID(r.Context()),
		Retryable: status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout,
	})
}

type traceContextKey struct{}

func traceID(ctx context.Context) string {
	value, _ := ctx.Value(traceContextKey{}).(string)
	return value
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (a *API) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		trace := r.Header.Get("X-Request-ID")
		if !safeTraceID.MatchString(trace) {
			trace = newID("trace")
		}
		ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), traceContextKey{}, trace), 15*time.Second)
		defer cancel()
		w.Header().Set("X-Request-ID", trace)
		writer := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error("request panic", "trace_id", trace)
				if writer.status == http.StatusOK {
					a.problem(writer, r.WithContext(ctx), http.StatusInternalServerError, "INTERNAL_ERROR", "Internal error", "The service could not complete the request.")
				}
			}
			a.logger.Info("http request", "method", r.Method, "path", r.URL.Path,
				"status", writer.status, "duration_ms", time.Since(start).Milliseconds(), "trace_id", trace)
		}()
		request := r.WithContext(ctx)
		if !a.authorized(request) {
			a.problem(writer, request, http.StatusUnauthorized, "AUTHENTICATION_REQUIRED", "Authentication required", "A valid bearer token is required.")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (a *API) authorized(r *http.Request) bool {
	if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		return true
	}
	expected := ""
	if strings.HasPrefix(r.URL.Path, "/internal/") {
		expected = a.auth.WorkerToken
		if expected == "" {
			return false
		}
	} else if a.auth.RequireAPIAuth {
		expected = a.auth.APIToken
	} else {
		return true
	}
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if provided == "" || len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func problemCategory(status int, code string) string {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "authorization"
	case status >= 500:
		return "platform"
	case strings.Contains(code, "TARGET"):
		return "target"
	default:
		return "configuration"
	}
}

func createdStatus(replayed bool) int {
	if replayed {
		return http.StatusOK
	}
	return http.StatusCreated
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	writeJSONStatus(w, status, "application/json", value)
}

func writeJSONStatus(w http.ResponseWriter, status int, contentType string, value any) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
