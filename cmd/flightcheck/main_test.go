package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	controlplane "github.com/Arshiamk/fhir-flightcheck/services/control-plane"
)

func TestCLIWorkflow(t *testing.T) {
	var healthy atomic.Bool
	healthy.Store(true)
	fhir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/fhir+json")
		version := "5.0.0"
		if healthy.Load() {
			version = "4.0.1"
		}
		_, _ = io.WriteString(w, `{"resourceType":"CapabilityStatement","fhirVersion":"`+version+`"}`)
	}))
	defer fhir.Close()
	signer, err := controlplane.NewEphemeralSigner()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := controlplane.LoadRuleCatalog("../../packages/rule-packs")
	if err != nil {
		t.Fatal(err)
	}
	repository := controlplane.NewMemoryRepository()
	service := &controlplane.Service{
		Repository: repository, Signer: signer, Catalog: catalog,
		Checker: controlplane.CapabilityChecker{
			Policy: controlplane.URLPolicy{AllowLocalDemo: true}, Timeout: time.Second,
		},
	}
	dispatchCtx, stopDispatch := context.WithCancel(t.Context())
	defer stopDispatch()
	go (&controlplane.OutboxDispatcher{
		Repository: repository, Publisher: &completionPublisher{service: service, catalog: catalog, healthy: &healthy},
		Interval: 5 * time.Millisecond,
	}).Run(dispatchCtx)
	api := httptest.NewServer(controlplane.NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil))))
	defer api.Close()

	configPath := filepath.Join(t.TempDir(), "flightcheck.json")
	reportPath := filepath.Join(t.TempDir(), "report.json")
	var stdout, stderr bytes.Buffer
	invoke := func(args ...string) int {
		stdout.Reset()
		stderr.Reset()
		return runCLI(args, &stdout, &stderr)
	}
	if code := invoke("init", "--api", api.URL, "--name", "CLI test", "--config", configPath); code != exitOK {
		t.Fatalf("init exit %d: %s", code, stderr.String())
	}
	if code := invoke("target", "add", "--config", configPath, "--name", "local", "--url", fhir.URL, "--allow-local-demo"); code != exitOK {
		t.Fatalf("target add exit %d: %s", code, stderr.String())
	}
	if code := invoke("run", "--config", configPath, "--output", reportPath); code != exitOK {
		t.Fatalf("run exit %d: %s", code, stderr.String())
	}
	if code := invoke("report", "verify", "--config", configPath, reportPath); code != exitOK {
		t.Fatalf("verify exit %d: %s", code, stderr.String())
	}
	if code := invoke("baseline", "set", "--config", configPath); code != exitOK {
		t.Fatalf("baseline exit %d: %s", code, stderr.String())
	}
	if code := invoke("ci", "--config", configPath, "--against", "baseline"); code != exitOK {
		t.Fatalf("ci exit %d: %s", code, stderr.String())
	}
	healthy.Store(false)
	if code := invoke("ci", "--config", configPath, "--against", "baseline"); code != exitGateFailed {
		t.Fatalf("regressed ci exit %d: %s", code, stderr.String())
	}
}

func TestReportVerifyRejectsTampering(t *testing.T) {
	signer, _ := controlplane.NewEphemeralSigner()
	report := controlplane.Report{
		SchemaVersion: controlplane.SchemaVersion, ReportID: "report_123", RunID: "run_123",
		ManifestSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Decision:       "ready", Coverage: controlplane.Coverage{}, Findings: []controlplane.Finding{},
	}
	if err := signer.Sign(&report); err != nil {
		t.Fatal(err)
	}
	report.Decision = "not_ready"
	body, _ := json.Marshal(report)
	path := filepath.Join(t.TempDir(), "tampered.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"report", "verify", "--key", signer.PublicKeyBase64(), path}, &stdout, &stderr)
	if code != exitVerifyFailed {
		t.Fatalf("exit = %d, want %d; stdout=%s stderr=%s", code, exitVerifyFailed, stdout.String(), stderr.String())
	}
}

func TestCLIStableUsageExit(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := runCLI([]string{"unknown"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("exit = %d, want %d", code, exitUsage)
	}
}

type completionPublisher struct {
	service *controlplane.Service
	catalog *controlplane.RuleCatalog
	healthy *atomic.Bool
}

func (p *completionPublisher) Publish(ctx context.Context, _ string, _ string, payload []byte) error {
	var job controlplane.EvaluationJob
	if err := json.Unmarshal(payload, &job); err != nil {
		return err
	}
	now := time.Unix(1_700_000_000, 0).UTC()
	input := controlplane.CompletionInput{JobID: job.JobID, RunID: job.Manifest.RunID}
	for index, rule := range p.catalog.Rules() {
		evidenceID := "evidence_" + rule.ID
		outcome := "pass"
		if !p.healthy.Load() && index == 0 {
			outcome = "fail"
		}
		input.Findings = append(input.Findings, controlplane.Finding{
			SchemaVersion: controlplane.SchemaVersion, FindingID: "finding_" + rule.ID,
			RunID: job.Manifest.RunID, RuleID: rule.ID, RuleVersion: rule.Version,
			Outcome: outcome, Severity: rule.Severity, Title: rule.Title,
			Summary: "The deterministic rule passed.", EvidenceRefs: []string{evidenceID},
			Remediation: rule.Remediation, ObservedAt: now,
		})
		input.Evidence = append(input.Evidence, controlplane.Evidence{
			SchemaVersion: controlplane.SchemaVersion, EvidenceID: evidenceID,
			RunID: job.Manifest.RunID, MediaType: "application/json",
			SHA256: fmt.Sprintf("%064x", index+1), SizeBytes: 2,
			StorageURI:      "urn:sha256:" + fmt.Sprintf("%064x", index+1),
			RedactionStatus: "not-required", SourceRuleID: rule.ID, CreatedAt: now,
		})
	}
	_, _, err := p.service.CompleteJob(ctx, input)
	return err
}
