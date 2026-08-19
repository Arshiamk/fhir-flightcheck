package controlplane

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var credentialReference = regexp.MustCompile(`^(?:none|[a-z][a-z0-9+.-]*:[^\s]{1,240})$`)

type Service struct {
	Repository     Repository
	Checker        CapabilityChecker
	Signer         *Signer
	Catalog        *RuleCatalog
	JobSubject     string
	OrganizationID string
	FixtureDir     string
	Now            func() time.Time
}

type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

func invalid(message string) error {
	return &ValidationError{Err: errors.New(message)}
}

type CreateTargetInput struct {
	Name                string `json:"name"`
	BaseURL             string `json:"baseUrl"`
	CredentialRef       string `json:"credentialRef"`
	AllowPrivateNetwork bool   `json:"allowPrivateNetwork"`
}

type CreateRunInput struct {
	TargetID string `json:"targetId"`
	Profile  string `json:"profile"`
}

func (s *Service) CreateProject(ctx context.Context, name, key string) (Project, bool, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 120 {
		return Project{}, false, invalid("project name must contain 2 to 120 characters")
	}
	now := s.now()
	value := Project{ID: newID("project"), Name: name, CreatedAt: now}
	idem := makeIdempotency("create-project", key, struct{ Name string }{name})
	return s.Repository.CreateProject(ctx, value, idem)
}

func (s *Service) CreateTarget(ctx context.Context, projectID string, input CreateTargetInput, key string) (Target, bool, error) {
	if _, err := s.Repository.GetProject(ctx, projectID); err != nil {
		return Target{}, false, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if input.Name == "" {
		return Target{}, false, invalid("target name is required")
	}
	parsed, err := s.Checker.Policy.Validate(ctx, input.BaseURL, input.AllowPrivateNetwork)
	if err != nil {
		return Target{}, false, &ValidationError{Err: err}
	}
	input.BaseURL = parsed.String()
	credentialRef := strings.TrimSpace(input.CredentialRef)
	if credentialRef == "" {
		credentialRef = "none"
	}
	if !credentialReference.MatchString(credentialRef) {
		return Target{}, false, invalid("credentialRef must be 'none' or an external secret reference such as env:NAME")
	}
	value := Target{
		ID: newID("target"), ProjectID: projectID, Name: input.Name, BaseURL: input.BaseURL,
		FHIRVersion: "4.0.1", CredentialRef: credentialRef,
		AllowPrivateNetwork: input.AllowPrivateNetwork, CreatedAt: s.now(),
	}
	idem := makeIdempotency("create-target:"+projectID, key, input)
	return s.Repository.CreateTarget(ctx, value, idem)
}

func (s *Service) CreateRun(ctx context.Context, projectID string, input CreateRunInput, key string) (Run, bool, error) {
	target, err := s.Repository.GetTarget(ctx, projectID, input.TargetID)
	if err != nil {
		return Run{}, false, err
	}
	if input.Profile == "" {
		input.Profile = "startup-r4"
	}
	if input.Profile != "startup-r4" {
		return Run{}, false, invalid("only the startup-r4 profile is currently supported")
	}
	now := s.now()
	runID := newID("run")
	if s.Catalog == nil {
		return Run{}, false, errors.New("rule catalog is unavailable")
	}
	run := Run{
		ID: runID, ProjectID: projectID, TargetID: target.ID, Status: "queued", CreatedAt: now,
		Manifest: RunManifest{
			SchemaVersion: SchemaVersion, RunID: runID, OrganizationID: s.organizationID(),
			ProjectID: projectID, Profile: input.Profile, CreatedAt: now,
			RuleVersions: s.Catalog.Versions(),
			Target: ManifestTarget{
				ID: target.ID, BaseURL: target.BaseURL, FHIRVersion: target.FHIRVersion,
				CredentialRef: "none", AllowPrivateNetwork: target.AllowPrivateNetwork,
			},
		},
	}
	fixture, fixtureVersion := s.loadFixture()
	if fixtureVersion != "" {
		run.Manifest.FixtureVersion = fixtureVersion
	}
	job := EvaluationJob{
		JobID: newID("job"), Manifest: run.Manifest, Fixture: fixture,
		Capabilities: s.Catalog.Capabilities(), Attempt: 1, MaxAttempts: 3,
	}
	payload, err := EncodeJob(job)
	if err != nil {
		return Run{}, false, fmt.Errorf("encode evaluation job: %w", err)
	}
	jobRecord := JobRecord{
		Job: job, OrganizationID: s.organizationID(), RunID: runID,
		Status: "queued", CreatedAt: now,
	}
	subject := s.JobSubject
	if subject == "" {
		subject = DefaultJobSubject
	}
	outbox := OutboxMessage{
		ID: newID("outbox"), OrganizationID: s.organizationID(), Subject: subject,
		Payload: payload, AvailableAt: now,
	}
	idem := makeIdempotency("create-run:"+projectID, key, input)
	return s.Repository.CreateQueuedRun(ctx, run, jobRecord, outbox, idem)
}

func (s *Service) CompleteJob(ctx context.Context, input CompletionInput) (Report, bool, error) {
	job, err := s.Repository.GetJob(ctx, input.JobID)
	if err != nil {
		return Report{}, false, err
	}
	if input.RunID == "" || input.RunID != job.RunID || input.RunID != job.Job.Manifest.RunID {
		return Report{}, false, ErrStaleCompletion
	}
	findings := append([]Finding(nil), input.Findings...)
	evidence := append([]Evidence(nil), input.Evidence...)
	sort.Slice(findings, func(i, j int) bool { return findings[i].RuleID < findings[j].RuleID })
	sort.Slice(evidence, func(i, j int) bool { return evidence[i].EvidenceID < evidence[j].EvidenceID })
	if err := s.validateCompletion(job, findings, evidence); err != nil {
		return Report{}, false, err
	}
	canonical, err := json.Marshal(CompletionInput{JobID: input.JobID, RunID: input.RunID, Findings: findings, Evidence: evidence})
	if err != nil {
		return Report{}, false, err
	}
	completionHash := sha256.Sum256(canonical)
	manifestJSON, err := json.Marshal(job.Job.Manifest)
	if err != nil {
		return Report{}, false, err
	}
	manifestHash := sha256.Sum256(manifestJSON)
	report := Report{
		SchemaVersion: SchemaVersion, ReportID: newID("report"), RunID: input.RunID,
		ManifestSHA256: hex.EncodeToString(manifestHash[:]), Decision: policyDecision(findings),
		Coverage: Coverage{Selected: len(job.Job.Manifest.RuleVersions), Completed: len(findings)},
		Findings: findings, CreatedAt: s.now(),
	}
	completeCoverage := len(findings) == len(job.Job.Manifest.RuleVersions)
	if !completeCoverage {
		report.Decision = "incomplete"
	}
	if s.Signer == nil {
		return Report{}, false, errors.New("report signer is unavailable")
	}
	if completeCoverage {
		if err := s.Signer.Sign(&report); err != nil {
			return Report{}, false, err
		}
	}
	return s.Repository.CompleteJob(ctx, input.JobID, hex.EncodeToString(completionHash[:]), findings, evidence, report, s.now())
}

func (s *Service) validateCompletion(job JobRecord, findings []Finding, evidence []Evidence) error {
	selected := job.Job.Manifest.RuleVersions
	if len(findings) > len(selected) {
		return invalid(fmt.Sprintf("completion contains %d findings for %d selected rules", len(findings), len(selected)))
	}
	rules := s.Catalog.ByID()
	evidenceIDs := make(map[string]Evidence, len(evidence))
	for _, item := range evidence {
		if item.RunID != job.RunID || item.EvidenceID == "" || len(item.SHA256) != 64 {
			return invalid("completion contains invalid evidence")
		}
		if _, exists := evidenceIDs[item.EvidenceID]; exists {
			return invalid("completion contains duplicate evidence IDs")
		}
		evidenceIDs[item.EvidenceID] = item
	}
	seen := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		version, selectedRule := selected[finding.RuleID]
		rule, knownRule := rules[finding.RuleID]
		if !selectedRule || !knownRule || finding.RunID != job.RunID || finding.RuleVersion != version {
			return invalid("completion contains a stale or unselected finding")
		}
		if finding.Severity != rule.Severity || finding.Title != rule.Title || finding.Remediation != rule.Remediation {
			return invalid("completion finding metadata does not match the pinned rule")
		}
		if _, exists := seen[finding.RuleID]; exists {
			return invalid("completion contains duplicate rule findings")
		}
		seen[finding.RuleID] = struct{}{}
		for _, evidenceID := range finding.EvidenceRefs {
			if _, exists := evidenceIDs[evidenceID]; !exists {
				return invalid("completion finding references missing evidence")
			}
		}
	}
	return nil
}

func policyDecision(findings []Finding) string {
	incomplete, conditional := false, false
	for _, finding := range findings {
		if finding.Outcome == "fail" && (finding.Severity == "critical" || finding.Severity == "high") {
			return "not_ready"
		}
		switch finding.Outcome {
		case "platform_error", "inconclusive":
			incomplete = true
		case "fail", "warning":
			conditional = true
		}
	}
	if incomplete {
		return "incomplete"
	}
	if conditional {
		return "conditional"
	}
	return "ready"
}

func (s *Service) SetBaseline(ctx context.Context, projectID, runID, key string) (Baseline, bool, error) {
	run, err := s.Repository.GetRun(ctx, runID)
	if err != nil {
		return Baseline{}, false, err
	}
	if run.ProjectID != projectID {
		return Baseline{}, false, ErrNotFound
	}
	report, err := s.Repository.GetReportByRun(ctx, runID)
	if err != nil {
		return Baseline{}, false, invalid("only completed runs can be baselines")
	}
	if report.Signature == nil || report.Coverage.Completed != report.Coverage.Selected || report.Decision == "incomplete" {
		return Baseline{}, false, invalid("only complete signed reports can be baselines")
	}
	value := Baseline{ProjectID: projectID, RunID: runID, ReportID: report.ReportID, SetAt: s.now()}
	idem := makeIdempotency("set-baseline:"+projectID, key, struct{ RunID string }{runID})
	return s.Repository.SetBaseline(ctx, value, idem)
}

func (s *Service) GetProject(ctx context.Context, id string) (Project, error) {
	return s.Repository.GetProject(ctx, id)
}

func (s *Service) GetTarget(ctx context.Context, projectID, id string) (Target, error) {
	return s.Repository.GetTarget(ctx, projectID, id)
}

func (s *Service) GetRun(ctx context.Context, id string) (Run, error) {
	return s.Repository.GetRun(ctx, id)
}

func (s *Service) GetReport(ctx context.Context, runID string) (Report, error) {
	return s.Repository.GetReportByRun(ctx, runID)
}

func (s *Service) GetBaseline(ctx context.Context, projectID string) (Baseline, error) {
	return s.Repository.GetBaseline(ctx, projectID)
}

func (s *Service) loadFixture() (map[string]any, string) {
	if s.FixtureDir == "" {
		return map[string]any{}, ""
	}
	path := filepath.Join(s.FixtureDir, "healthy.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}, ""
	}
	var fixture map[string]any
	if err := json.Unmarshal(data, &fixture); err != nil {
		return map[string]any{}, ""
	}
	return fixture, "synthea-v1"
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) organizationID() string {
	if s.OrganizationID != "" {
		return s.OrganizationID
	}
	return "org_local"
}

func makeIdempotency(operation, key string, request any) Idempotency {
	body, _ := json.Marshal(request)
	sum := sha256.Sum256(body)
	return Idempotency{Operation: operation, Key: key, RequestHash: hex.EncodeToString(sum[:])}
}

func newID(prefix string) string {
	var random [10]byte
	if _, err := rand.Read(random[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return prefix + "_" + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(random[:]))
}

func NormalizeAPIURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("API URL must be an absolute HTTP or HTTPS URL")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}
