package controlplane

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServiceWithFixture(t *testing.T, fixtureDir string) (*Service, *httptest.Server) {
	t.Helper()
	fhir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/metadata" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/fhir+json")
		_, _ = io.WriteString(w, `{"resourceType":"CapabilityStatement","fhirVersion":"4.0.1"}`)
	}))
	signer, err := NewEphemeralSigner()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := LoadRuleCatalog("../../packages/rule-packs")
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{
		Repository: NewMemoryRepository(),
		Signer:     signer,
		Catalog:    catalog,
		FixtureDir: fixtureDir,
		Checker:    CapabilityChecker{Policy: URLPolicy{AllowLocalDemo: true}, Timeout: time.Second},
	}
	return svc, fhir
}

func createRunForFixtureTest(t *testing.T, svc *Service, fhirURL string) EvaluationJob {
	t.Helper()
	ctx := t.Context()
	project, _, err := svc.CreateProject(ctx, "fixture-test-project", "key-proj-fixture")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	target, _, err := svc.CreateTarget(ctx, project.ID, CreateTargetInput{
		Name: "test", BaseURL: fhirURL, AllowPrivateNetwork: true,
	}, "key-target-fixture")
	if err != nil {
		t.Fatalf("create target: %v", err)
	}
	_, _, err = svc.CreateRun(ctx, project.ID, CreateRunInput{TargetID: target.ID, Profile: "startup-r4"}, "key-run-fixture")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	repo := svc.Repository.(*MemoryRepository)
	messages, err := repo.ClaimOutbox(ctx, 1, time.Second)
	if err != nil || len(messages) != 1 {
		t.Fatalf("claim outbox: %v, %d messages", err, len(messages))
	}
	var job EvaluationJob
	if err := json.Unmarshal(messages[0].Payload, &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	return job
}

func TestCreateRunLoadsFixtureWhenDirSet(t *testing.T) {
	svc, fhir := newTestServiceWithFixture(t, "../../fixtures/synthea")
	defer fhir.Close()

	job := createRunForFixtureTest(t, svc, fhir.URL)

	if len(job.Fixture) == 0 {
		t.Fatal("expected non-empty fixture in job, got empty map")
	}
	if _, ok := job.Fixture["scenario"]; !ok {
		t.Errorf("fixture missing 'scenario' key; got keys: %v", fixtureKeys(job.Fixture))
	}
	if job.Manifest.FixtureVersion != "synthea-v1" {
		t.Errorf("expected FixtureVersion=synthea-v1, got %q", job.Manifest.FixtureVersion)
	}
}

func TestCreateRunEmptyFixtureWhenDirMissing(t *testing.T) {
	svc, fhir := newTestServiceWithFixture(t, "")
	defer fhir.Close()

	job := createRunForFixtureTest(t, svc, fhir.URL)

	if len(job.Fixture) != 0 {
		t.Errorf("expected empty fixture, got %d keys", len(job.Fixture))
	}
	if job.Manifest.FixtureVersion != "" {
		t.Errorf("expected no FixtureVersion, got %q", job.Manifest.FixtureVersion)
	}
}

func TestCreateRunEmptyFixtureWhenDirInvalid(t *testing.T) {
	svc, fhir := newTestServiceWithFixture(t, "/nonexistent/path/fixtures")
	defer fhir.Close()

	job := createRunForFixtureTest(t, svc, fhir.URL)

	if len(job.Fixture) != 0 {
		t.Errorf("expected empty fixture for invalid dir, got %d keys", len(job.Fixture))
	}
}

func fixtureKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
