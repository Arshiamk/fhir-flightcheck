package controlplane

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"
)

func TestPostgresRepositoryIdempotencyAndTenantIsolation(t *testing.T) {
	databaseURL := os.Getenv("FLIGHTCHECK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set FLIGHTCHECK_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}
	suffix := newID("tenant")
	first, err := NewPostgresRepository(t.Context(), databaseURL, "org_a_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewPostgresRepository(t.Context(), databaseURL, "org_b_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	now := time.Now().UTC()
	projectID := newID("project")
	idem := Idempotency{Operation: "test-project", Key: "same-key-123", RequestHash: "hash-a"}
	created, replay, err := first.CreateProject(t.Context(), Project{ID: projectID, Name: "Tenant A", CreatedAt: now}, idem)
	if err != nil || replay {
		t.Fatalf("first create: replay=%v err=%v", replay, err)
	}
	replayed, replay, err := first.CreateProject(t.Context(), Project{ID: newID("project"), Name: "Tenant A", CreatedAt: now}, idem)
	if err != nil || !replay || replayed.ID != created.ID {
		t.Fatalf("idempotent replay: value=%+v replay=%v err=%v", replayed, replay, err)
	}
	conflict := idem
	conflict.RequestHash = "hash-b"
	if _, _, err := first.CreateProject(t.Context(), Project{ID: newID("project"), Name: "Changed", CreatedAt: now}, conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting idempotency error = %v", err)
	}
	if _, _, err := second.CreateProject(t.Context(), Project{ID: projectID, Name: "Tenant B", CreatedAt: now},
		Idempotency{Operation: "test-project", Key: "same-key-123", RequestHash: "hash-b"}); err != nil {
		t.Fatalf("same IDs in separate tenant should succeed: %v", err)
	}
	firstValue, err := first.GetProject(t.Context(), projectID)
	if err != nil || firstValue.Name != "Tenant A" {
		t.Fatalf("tenant A read = %+v, %v", firstValue, err)
	}
	secondValue, err := second.GetProject(t.Context(), projectID)
	if err != nil || secondValue.Name != "Tenant B" {
		t.Fatalf("tenant B read = %+v, %v", secondValue, err)
	}
}

// TestPostgresConnectionPoolDoesNotLeakOrganizationID creates two repositories with
// distinct organization IDs and concurrently attempts cross-tenant project lookups,
// confirming that RLS prevents any data leaking between connection-pool connections.
func TestPostgresConnectionPoolDoesNotLeakOrganizationID(t *testing.T) {
	databaseURL := os.Getenv("FLIGHTCHECK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set FLIGHTCHECK_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	suffix := newID("pool")
	orgA, err := NewPostgresRepository(t.Context(), databaseURL, "pool_org_a_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer orgA.Close()
	orgB, err := NewPostgresRepository(t.Context(), databaseURL, "pool_org_b_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer orgB.Close()

	now := time.Now().UTC()

	projectA := newID("proj")
	if _, _, err := orgA.CreateProject(t.Context(), Project{ID: projectA, Name: "Pool A", CreatedAt: now},
		Idempotency{Operation: "pool-test", Key: projectA, RequestHash: "ha"}); err != nil {
		t.Fatal(err)
	}

	projectB := newID("proj")
	if _, _, err := orgB.CreateProject(t.Context(), Project{ID: projectB, Name: "Pool B", CreatedAt: now},
		Idempotency{Operation: "pool-test", Key: projectB, RequestHash: "hb"}); err != nil {
		t.Fatal(err)
	}

	const concurrency = 50
	var wg sync.WaitGroup
	errorsA := make([]error, concurrency)
	errorsB := make([]error, concurrency)

	// Org A tries to read org B's project.
	for i := range concurrency {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errorsA[idx] = orgA.GetProject(t.Context(), projectB)
		}(i)
	}

	// Org B tries to read org A's project.
	for i := range concurrency {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errorsB[idx] = orgB.GetProject(t.Context(), projectA)
		}(i)
	}

	wg.Wait()

	for i, err := range errorsA {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("org A goroutine %d: expected ErrNotFound reading org B project, got %v", i, err)
		}
	}
	for i, err := range errorsB {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("org B goroutine %d: expected ErrNotFound reading org A project, got %v", i, err)
		}
	}
}

// TestPostgresRunsAndJobsAreIsolatedByOrganization creates a run and job in org A and
// confirms that org B cannot retrieve the run (ErrNotFound), while org A can.
func TestPostgresRunsAndJobsAreIsolatedByOrganization(t *testing.T) {
	databaseURL := os.Getenv("FLIGHTCHECK_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set FLIGHTCHECK_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	suffix := newID("rls")
	orgA, err := NewPostgresRepository(t.Context(), databaseURL, "rls_org_a_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer orgA.Close()
	orgB, err := NewPostgresRepository(t.Context(), databaseURL, "rls_org_b_"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	defer orgB.Close()

	now := time.Now().UTC()

	// Create a project and target in org A so the run has valid foreign keys.
	projectID := newID("proj")
	if _, _, err := orgA.CreateProject(t.Context(), Project{ID: projectID, Name: "RLS A", CreatedAt: now},
		Idempotency{Operation: "rls-project", Key: projectID, RequestHash: "hp"}); err != nil {
		t.Fatal(err)
	}
	targetID := newID("target")
	if _, _, err := orgA.CreateTarget(t.Context(), Target{ID: targetID, ProjectID: projectID, Name: "T", BaseURL: "https://rls.example/fhir", FHIRVersion: "4.0.1", CredentialRef: "secret:rls", CreatedAt: now},
		Idempotency{Operation: "rls-target", Key: targetID, RequestHash: "ht"}); err != nil {
		t.Fatal(err)
	}

	runID := newID("run")
	jobID := newID("job")
	outboxID := newID("outbox")
	manifest := RunManifest{
		SchemaVersion:  SchemaVersion,
		RunID:          runID,
		OrganizationID: "rls_org_a_" + suffix,
		ProjectID:      projectID,
		Target: ManifestTarget{
			ID:            targetID,
			BaseURL:       "https://rls.example/fhir",
			FHIRVersion:   "4.0.1",
			CredentialRef: "secret:rls",
		},
		Profile:      "startup-r4",
		RuleVersions: map[string]string{},
		CreatedAt:    now,
	}
	run := Run{ID: runID, ProjectID: projectID, TargetID: targetID, Status: "queued", Manifest: manifest, CreatedAt: now}
	job := JobRecord{
		Job:            EvaluationJob{JobID: jobID, Manifest: manifest, Fixture: map[string]any{}, Capabilities: []string{"fixtures"}, Attempt: 1, MaxAttempts: 3},
		OrganizationID: "rls_org_a_" + suffix,
		RunID:          runID,
		Status:         "queued",
		CreatedAt:      now,
	}
	payload, _ := json.Marshal(map[string]string{"runId": runID})
	outbox := OutboxMessage{ID: outboxID, OrganizationID: "rls_org_a_" + suffix, Subject: "run.queued", Payload: payload, AvailableAt: now}

	if _, _, err := orgA.CreateQueuedRun(t.Context(), run, job, outbox,
		Idempotency{Operation: "rls-run", Key: runID, RequestHash: "hr"}); err != nil {
		t.Fatal(err)
	}

	// Org A can read its own run.
	if _, err := orgA.GetRun(t.Context(), runID); err != nil {
		t.Fatalf("org A GetRun: expected success, got %v", err)
	}

	// Org B must not be able to read org A's run.
	if _, err := orgB.GetRun(t.Context(), runID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("org B GetRun: expected ErrNotFound, got %v", err)
	}

	// Org B must not be able to read org A's job either.
	if _, err := orgB.GetJob(t.Context(), jobID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("org B GetJob: expected ErrNotFound, got %v", err)
	}
}
