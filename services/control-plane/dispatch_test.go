package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type recordingPublisher struct {
	payloads [][]byte
	fail     bool
}

func (p *recordingPublisher) Publish(_ context.Context, _, _ string, payload []byte) error {
	if p.fail {
		return errors.New("NATS unavailable")
	}
	p.payloads = append(p.payloads, append([]byte(nil), payload...))
	return nil
}

func TestJobEncodingMatchesWorkerContract(t *testing.T) {
	job := EvaluationJob{
		JobID: "job_123", Manifest: RunManifest{
			SchemaVersion: SchemaVersion, RunID: "run_123", OrganizationID: "org_local",
			ProjectID: "project_123", Profile: "startup-r4",
			RuleVersions: map[string]string{"fhir.discovery.capability": "1.0.0"},
			Target:       ManifestTarget{ID: "target_123", BaseURL: "https://example.test/fhir", FHIRVersion: "4.0.1", CredentialRef: "none"},
			CreatedAt:    time.Unix(1_700_000_000, 0).UTC(),
		},
		Fixture: map[string]any{}, Capabilities: []string{"network"},
		Attempt: 1, MaxAttempts: 3,
	}
	body, err := EncodeJob(job)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"jobId", "manifest", "fixture", "capabilities", "attempt", "maxAttempts"} {
		if _, ok := wire[key]; !ok {
			t.Errorf("worker field %q missing from %s", key, body)
		}
	}
	manifest := wire["manifest"].(map[string]any)
	target := manifest["target"].(map[string]any)
	if target["credentialRef"] != "none" {
		t.Fatalf("job exposed credential reference: %v", target["credentialRef"])
	}
}

func TestOutboxDispatchIsRetryableAndIdempotent(t *testing.T) {
	repository := NewMemoryRepository()
	message := OutboxMessage{
		ID: "outbox_123", OrganizationID: "org_local", Subject: DefaultJobSubject,
		Payload: json.RawMessage(`{"jobId":"job_123"}`), AvailableAt: time.Now().Add(-time.Second),
	}
	repository.outbox[message.ID] = message
	publisher := &recordingPublisher{fail: true}
	dispatcher := &OutboxDispatcher{Repository: repository, Publisher: publisher}
	if err := dispatcher.DispatchOnce(t.Context()); err != nil {
		t.Fatalf("transient publish should be retained, got %v", err)
	}
	if len(repository.outbox) != 1 || repository.outbox[message.ID].Attempts != 1 {
		t.Fatalf("failed message was not retained: %+v", repository.outbox)
	}
	repository.outbox[message.ID] = OutboxMessage{
		ID: message.ID, OrganizationID: message.OrganizationID, Subject: message.Subject,
		Payload: message.Payload, Attempts: 1, AvailableAt: time.Now().Add(-time.Second),
	}
	publisher.fail = false
	if err := dispatcher.DispatchOnce(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(publisher.payloads) != 1 || len(repository.outbox) != 0 {
		t.Fatalf("successful publish state: payloads=%d outbox=%d", len(publisher.payloads), len(repository.outbox))
	}
	if err := dispatcher.DispatchOnce(t.Context()); err != nil || len(publisher.payloads) != 1 {
		t.Fatal("published outbox row was dispatched twice")
	}
}

func TestBlockerFirstPolicy(t *testing.T) {
	if got := policyDecision([]Finding{{Outcome: "platform_error", Severity: "medium"}, {Outcome: "fail", Severity: "critical"}}); got != "not_ready" {
		t.Fatalf("critical blocker decision = %q", got)
	}
	if got := policyDecision([]Finding{{Outcome: "inconclusive", Severity: "low"}}); got != "incomplete" {
		t.Fatalf("inconclusive decision = %q", got)
	}
	if got := policyDecision([]Finding{{Outcome: "fail", Severity: "medium"}}); got != "conditional" {
		t.Fatalf("non-blocker failure decision = %q", got)
	}
}

func TestIncompleteCoverageIsPersistedUnsigned(t *testing.T) {
	catalog, err := LoadRuleCatalog("../../packages/rule-packs")
	if err != nil {
		t.Fatal(err)
	}
	signer, _ := NewEphemeralSigner()
	repository := NewMemoryRepository()
	now := time.Now().UTC()
	project := Project{ID: "project_partial", Name: "Partial", CreatedAt: now}
	_, _, _ = repository.CreateProject(t.Context(), project, Idempotency{Operation: "project", Key: "project-key", RequestHash: "a"})
	target := Target{
		ID: "target_partial", ProjectID: project.ID, Name: "FHIR", BaseURL: "https://example.test/fhir",
		FHIRVersion: "4.0.1", CredentialRef: "none", CreatedAt: now,
	}
	_, _, _ = repository.CreateTarget(t.Context(), target, Idempotency{Operation: "target", Key: "target-key", RequestHash: "a"})
	service := &Service{Repository: repository, Signer: signer, Catalog: catalog}
	run, _, err := service.CreateRun(t.Context(), project.ID, CreateRunInput{TargetID: target.ID, Profile: "startup-r4"}, "run-key-123")
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := repository.ClaimOutbox(t.Context(), 1, time.Second)
	var job EvaluationJob
	if err := json.Unmarshal(messages[0].Payload, &job); err != nil {
		t.Fatal(err)
	}
	completion := passingCompletion(catalog, job)
	completion.Findings = completion.Findings[:len(completion.Findings)-1]
	completion.Evidence = completion.Evidence[:len(completion.Evidence)-1]
	report, _, err := service.CompleteJob(t.Context(), completion)
	if err != nil {
		t.Fatal(err)
	}
	if report.Decision != "incomplete" || report.Signature != nil ||
		report.Coverage.Completed >= report.Coverage.Selected {
		t.Fatalf("partial report was not honestly represented: %+v", report)
	}
	storedRun, _ := repository.GetRun(t.Context(), run.ID)
	if storedRun.Status != "completed" {
		t.Fatalf("partial terminal run status = %q", storedRun.Status)
	}
}
