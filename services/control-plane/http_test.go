package controlplane

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAPIEndToEndAndIdempotency(t *testing.T) {
	fhir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/fhir+json")
		_, _ = io.WriteString(w, `{"resourceType":"CapabilityStatement","fhirVersion":"4.0.1"}`)
	}))
	defer fhir.Close()
	signer, err := NewEphemeralSigner()
	if err != nil {
		t.Fatal(err)
	}
	repository := NewMemoryRepository()
	catalog, err := LoadRuleCatalog("../../packages/rule-packs")
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{
		Repository: repository, Signer: signer, Catalog: catalog,
		Checker: CapabilityChecker{Policy: URLPolicy{AllowLocalDemo: true}, Timeout: time.Second},
	}
	server := httptest.NewServer(NewHandlerWithAuth(service, slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthConfig{WorkerToken: "worker-token-which-is-long-enough-123"}))
	defer server.Close()

	var project Project
	status := requestJSON(t, server.Client(), http.MethodPost, server.URL+"/v1/projects", "project-key-123",
		map[string]string{"name": "Demo project"}, &project)
	if status != http.StatusCreated {
		t.Fatalf("create project status = %d", status)
	}
	var replay Project
	status = requestJSON(t, server.Client(), http.MethodPost, server.URL+"/v1/projects", "project-key-123",
		map[string]string{"name": "Demo project"}, &replay)
	if status != http.StatusOK || replay.ID != project.ID {
		t.Fatalf("idempotent replay = %d %+v", status, replay)
	}
	var problem Problem
	status = requestJSON(t, server.Client(), http.MethodPost, server.URL+"/v1/projects", "project-key-123",
		map[string]string{"name": "Different project"}, &problem)
	if status != http.StatusConflict || problem.Code != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("conflict = %d %+v", status, problem)
	}

	var target Target
	status = requestJSON(t, server.Client(), http.MethodPost, server.URL+"/v1/projects/"+project.ID+"/targets", "target-key-123",
		CreateTargetInput{Name: "FHIR demo", BaseURL: fhir.URL, AllowPrivateNetwork: true}, &target)
	if status != http.StatusCreated {
		t.Fatalf("create target status = %d", status)
	}
	var run Run
	status = requestJSON(t, server.Client(), http.MethodPost, server.URL+"/v1/projects/"+project.ID+"/runs", "run-key-12345",
		CreateRunInput{TargetID: target.ID, Profile: "startup-r4"}, &run)
	if status != http.StatusCreated || run.Status != "queued" {
		t.Fatalf("create run = %d %+v", status, run)
	}
	messages, err := repository.ClaimOutbox(t.Context(), 1, time.Second)
	if err != nil || len(messages) != 1 {
		t.Fatalf("claim outbox: %v, %d messages", err, len(messages))
	}
	var job EvaluationJob
	if err := json.Unmarshal(messages[0].Payload, &job); err != nil {
		t.Fatal(err)
	}
	completion := passingCompletion(catalog, job)
	wrongRun := completion
	wrongRun.RunID = "run_wrong"
	var wrongRunProblem Problem
	status = requestJSONAuth(t, server.Client(), http.MethodPost,
		server.URL+"/internal/v1/jobs/"+job.JobID+"/complete", "", "worker-token-which-is-long-enough-123",
		wrongRun, &wrongRunProblem)
	if status != http.StatusConflict || wrongRunProblem.Code != "STALE_COMPLETION" {
		t.Fatalf("wrong-run completion = %d %+v", status, wrongRunProblem)
	}
	var completedReport Report
	status = requestJSONAuth(t, server.Client(), http.MethodPost,
		server.URL+"/internal/v1/jobs/"+job.JobID+"/complete", "", "worker-token-which-is-long-enough-123",
		completion, &completedReport)
	if status != http.StatusCreated || completedReport.Decision != "ready" {
		t.Fatalf("complete job = %d %+v", status, completedReport)
	}
	status = requestJSONAuth(t, server.Client(), http.MethodPost,
		server.URL+"/internal/v1/jobs/"+job.JobID+"/complete", "", "worker-token-which-is-long-enough-123",
		completion, &completedReport)
	if status != http.StatusOK {
		t.Fatalf("duplicate completion status = %d", status)
	}
	completion.Findings[0].Summary = "Conflicting duplicate completion."
	var staleProblem Problem
	status = requestJSONAuth(t, server.Client(), http.MethodPost,
		server.URL+"/internal/v1/jobs/"+job.JobID+"/complete", "", "worker-token-which-is-long-enough-123",
		completion, &staleProblem)
	if status != http.StatusConflict || staleProblem.Code != "STALE_COMPLETION" {
		t.Fatalf("conflicting completion = %d %+v", status, staleProblem)
	}
	var report Report
	status = requestJSON(t, server.Client(), http.MethodGet, server.URL+"/v1/runs/"+run.ID+"/report", "", nil, &report)
	if status != http.StatusOK || report.Decision != "ready" {
		t.Fatalf("report = %d %+v", status, report)
	}
	public, _ := ParsePublicKey(signer.PublicKeyBase64())
	if err := VerifyReport(report, public); err != nil {
		t.Fatalf("API report signature: %v", err)
	}
	var baseline Baseline
	status = requestJSON(t, server.Client(), http.MethodPut, server.URL+"/v1/projects/"+project.ID+"/baseline", "baseline-key-1",
		map[string]string{"runId": run.ID}, &baseline)
	if status != http.StatusCreated || baseline.RunID != run.ID {
		t.Fatalf("baseline = %d %+v", status, baseline)
	}
}

func TestAPIProblemsAndHealth(t *testing.T) {
	signer, _ := NewEphemeralSigner()
	service := &Service{Repository: NewMemoryRepository(), Signer: signer}
	server := httptest.NewServer(NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer server.Close()

	var health map[string]string
	if status := requestJSON(t, server.Client(), http.MethodGet, server.URL+"/healthz", "", nil, &health); status != http.StatusOK || health["status"] != "ok" {
		t.Fatalf("health response = %d %+v", status, health)
	}
	var problem Problem
	status := requestJSON(t, server.Client(), http.MethodPost, server.URL+"/v1/projects", "",
		map[string]string{"name": "demo"}, &problem)
	if status != http.StatusBadRequest || problem.Code != "INVALID_IDEMPOTENCY_KEY" || problem.TraceID == "" {
		t.Fatalf("problem response = %d %+v", status, problem)
	}
}

func TestAPIAuthenticationBoundary(t *testing.T) {
	signer, _ := NewEphemeralSigner()
	service := &Service{Repository: NewMemoryRepository(), Signer: signer}
	server := httptest.NewServer(NewHandlerWithAuth(service, slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthConfig{RequireAPIAuth: true, APIToken: "api-token-long-enough", WorkerToken: "worker-token-long-enough"}))
	defer server.Close()
	var problem Problem
	status := requestJSON(t, server.Client(), http.MethodGet, server.URL+"/v1/signing-key", "", nil, &problem)
	if status != http.StatusUnauthorized || problem.Category != "authorization" || problem.SchemaVersion != SchemaVersion {
		t.Fatalf("unauthorized problem = %d %+v", status, problem)
	}
	var key map[string]string
	status = requestJSONAuth(t, server.Client(), http.MethodGet, server.URL+"/v1/signing-key", "", "api-token-long-enough", nil, &key)
	if status != http.StatusOK || key["algorithm"] != "Ed25519" {
		t.Fatalf("authenticated response = %d %+v", status, key)
	}
	status = requestJSONAuth(t, server.Client(), http.MethodGet, server.URL+"/v1/signing-key", "", "worker-token-long-enough", nil, &problem)
	if status != http.StatusUnauthorized {
		t.Fatalf("worker token accessed public API: %d", status)
	}
	status = requestJSONAuth(t, server.Client(), http.MethodPost, server.URL+"/internal/v1/jobs/job_123/complete", "", "api-token-long-enough",
		map[string]any{}, &problem)
	if status != http.StatusUnauthorized {
		t.Fatalf("API token accessed worker endpoint: %d", status)
	}
}

func requestJSON(t *testing.T, client *http.Client, method, rawURL, key string, input, output any) int {
	return requestJSONAuth(t, client, method, rawURL, key, "", input, output)
}

func requestJSONAuth(t *testing.T, client *http.Client, method, rawURL, key, token string, input, output any) int {
	t.Helper()
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(t.Context(), method, rawURL, body)
	if err != nil {
		t.Fatal(err)
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if output != nil {
		if err := json.NewDecoder(response.Body).Decode(output); err != nil {
			t.Fatal(err)
		}
	}
	return response.StatusCode
}

func passingCompletion(catalog *RuleCatalog, job EvaluationJob) CompletionInput {
	now := time.Unix(1_700_000_000, 0).UTC()
	findings := make([]Finding, 0, len(catalog.Rules()))
	evidence := make([]Evidence, 0, len(catalog.Rules()))
	for index, rule := range catalog.Rules() {
		evidenceID := "evidence_" + rule.ID
		findings = append(findings, Finding{
			SchemaVersion: SchemaVersion, FindingID: "finding_" + rule.ID,
			RunID: job.Manifest.RunID, RuleID: rule.ID, RuleVersion: rule.Version,
			Outcome: "pass", Severity: rule.Severity, Title: rule.Title,
			Summary: "The deterministic rule passed.", EvidenceRefs: []string{evidenceID},
			Remediation: rule.Remediation, ObservedAt: now,
		})
		evidence = append(evidence, Evidence{
			SchemaVersion: SchemaVersion, EvidenceID: evidenceID, RunID: job.Manifest.RunID,
			MediaType: "application/json", SHA256: fmt.Sprintf("%064x", index+1), SizeBytes: 2,
			StorageURI:      "urn:sha256:" + fmt.Sprintf("%064x", index+1),
			RedactionStatus: "not-required", SourceRuleID: rule.ID, CreatedAt: now,
		})
	}
	return CompletionInput{JobID: job.JobID, RunID: job.Manifest.RunID, Findings: findings, Evidence: evidence}
}
