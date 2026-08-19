// Package controlplane implements the local Flightcheck API and its domain model.
package controlplane

import (
	"encoding/json"
	"time"
)

const SchemaVersion = "1.0.0"

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Target struct {
	ID                  string    `json:"id"`
	ProjectID           string    `json:"projectId"`
	Name                string    `json:"name"`
	BaseURL             string    `json:"baseUrl"`
	FHIRVersion         string    `json:"fhirVersion"`
	CredentialRef       string    `json:"credentialRef"`
	AllowPrivateNetwork bool      `json:"allowPrivateNetwork,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
}

type ManifestTarget struct {
	ID                  string `json:"id"`
	BaseURL             string `json:"baseUrl"`
	FHIRVersion         string `json:"fhirVersion"`
	CredentialRef       string `json:"credentialRef"`
	AllowPrivateNetwork bool   `json:"allowPrivateNetwork,omitempty"`
}

type RunManifest struct {
	SchemaVersion  string            `json:"schemaVersion"`
	RunID          string            `json:"runId"`
	OrganizationID string            `json:"organizationId"`
	ProjectID      string            `json:"projectId"`
	Target         ManifestTarget    `json:"target"`
	Profile        string            `json:"profile"`
	RuleVersions   map[string]string `json:"ruleVersions"`
	FixtureVersion string            `json:"fixtureVersion,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
}

type Run struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"projectId"`
	TargetID    string      `json:"targetId"`
	Status      string      `json:"status"`
	Manifest    RunManifest `json:"manifest"`
	ReportID    string      `json:"reportId,omitempty"`
	ErrorDetail string      `json:"errorDetail,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
	CompletedAt *time.Time  `json:"completedAt,omitempty"`
}

type Rule struct {
	SchemaVersion         string   `json:"schemaVersion"`
	ID                    string   `json:"id"`
	Version               string   `json:"version"`
	Title                 string   `json:"title"`
	Description           string   `json:"description"`
	Category              string   `json:"category"`
	Severity              string   `json:"severity"`
	Behavior              string   `json:"behavior"`
	Deterministic         bool     `json:"deterministic"`
	SupportedFHIRVersions []string `json:"supportedFhirVersions,omitempty"`
	Capabilities          []string `json:"capabilities"`
	TimeoutSeconds        int      `json:"timeoutSeconds"`
	Evaluator             string   `json:"evaluator"`
	StandardReferences    []string `json:"standardReferences,omitempty"`
	Remediation           string   `json:"remediation"`
	ReplacedBy            string   `json:"replacedBy,omitempty"`
}

type EvaluationJob struct {
	JobID        string         `json:"jobId"`
	Manifest     RunManifest    `json:"manifest"`
	Fixture      map[string]any `json:"fixture"`
	Capabilities []string       `json:"capabilities"`
	Attempt      int            `json:"attempt"`
	MaxAttempts  int            `json:"maxAttempts"`
}

type JobRecord struct {
	Job            EvaluationJob `json:"job"`
	OrganizationID string        `json:"organizationId"`
	RunID          string        `json:"runId"`
	Status         string        `json:"status"`
	CompletionHash string        `json:"completionHash,omitempty"`
	CreatedAt      time.Time     `json:"createdAt"`
	CompletedAt    *time.Time    `json:"completedAt,omitempty"`
}

type CompletionInput struct {
	JobID    string     `json:"jobId"`
	RunID    string     `json:"runId"`
	Findings []Finding  `json:"findings"`
	Evidence []Evidence `json:"evidence"`
}

type OutboxMessage struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	Subject        string          `json:"subject"`
	Payload        json.RawMessage `json:"payload"`
	Attempts       int             `json:"attempts"`
	AvailableAt    time.Time       `json:"availableAt"`
}

type Finding struct {
	SchemaVersion string    `json:"schemaVersion"`
	FindingID     string    `json:"findingId"`
	RunID         string    `json:"runId"`
	RuleID        string    `json:"ruleId"`
	RuleVersion   string    `json:"ruleVersion"`
	Outcome       string    `json:"outcome"`
	Severity      string    `json:"severity"`
	Title         string    `json:"title"`
	Summary       string    `json:"summary"`
	EvidenceRefs  []string  `json:"evidenceRefs"`
	Remediation   string    `json:"remediation"`
	ObservedAt    time.Time `json:"observedAt"`
}

type Evidence struct {
	SchemaVersion   string    `json:"schemaVersion"`
	EvidenceID      string    `json:"evidenceId"`
	RunID           string    `json:"runId"`
	MediaType       string    `json:"mediaType"`
	SHA256          string    `json:"sha256"`
	SizeBytes       int64     `json:"sizeBytes"`
	StorageURI      string    `json:"storageUri"`
	RedactionStatus string    `json:"redactionStatus"`
	SourceRuleID    string    `json:"sourceRuleId,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Coverage struct {
	Selected  int `json:"selected"`
	Completed int `json:"completed"`
}

type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"keyId"`
	Value     string `json:"value"`
}

type Report struct {
	SchemaVersion  string     `json:"schemaVersion"`
	ReportID       string     `json:"reportId"`
	RunID          string     `json:"runId"`
	ManifestSHA256 string     `json:"manifestSha256"`
	Decision       string     `json:"decision"`
	Coverage       Coverage   `json:"coverage"`
	Findings       []Finding  `json:"findings"`
	Signature      *Signature `json:"signature,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type Baseline struct {
	ProjectID string    `json:"projectId"`
	RunID     string    `json:"runId"`
	ReportID  string    `json:"reportId"`
	SetAt     time.Time `json:"setAt"`
}

type Problem struct {
	SchemaVersion     string `json:"schemaVersion"`
	Type              string `json:"type"`
	Title             string `json:"title"`
	Status            int    `json:"status"`
	Detail            string `json:"detail"`
	Instance          string `json:"instance"`
	Category          string `json:"category"`
	Code              string `json:"code"`
	TraceID           string `json:"traceId"`
	Retryable         bool   `json:"retryable"`
	RetryAfterSeconds *int   `json:"retryAfterSeconds,omitempty"`
	RunID             string `json:"runId,omitempty"`
}

type CheckResult struct {
	Finding  Finding
	Evidence Evidence
}
