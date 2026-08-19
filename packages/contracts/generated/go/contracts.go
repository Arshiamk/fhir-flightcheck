// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    rule, err := UnmarshalRule(bytes)
//    bytes, err = rule.Marshal()
//
//    runManifest, err := UnmarshalRunManifest(bytes)
//    bytes, err = runManifest.Marshal()
//
//    finding, err := UnmarshalFinding(bytes)
//    bytes, err = finding.Marshal()
//
//    evidence, err := UnmarshalEvidence(bytes)
//    bytes, err = evidence.Marshal()
//
//    report, err := UnmarshalReport(bytes)
//    bytes, err = report.Marshal()
//
//    aPIProblem, err := UnmarshalAPIProblem(bytes)
//    bytes, err = aPIProblem.Marshal()

package contracts

import "time"

import "encoding/json"

func UnmarshalRule(data []byte) (Rule, error) {
	var r Rule
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Rule) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalRunManifest(data []byte) (RunManifest, error) {
	var r RunManifest
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *RunManifest) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalFinding(data []byte) (Finding, error) {
	var r Finding
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Finding) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalEvidence(data []byte) (Evidence, error) {
	var r Evidence
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Evidence) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalReport(data []byte) (Report, error) {
	var r Report
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Report) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAPIProblem(data []byte) (APIProblem, error) {
	var r APIProblem
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *APIProblem) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Rule struct {
	Behavior              Behavior      `json:"behavior"`
	Capabilities          []Capability  `json:"capabilities"`
	Category              RuleCategory  `json:"category"`
	Description           string        `json:"description"`
	Deterministic         bool          `json:"deterministic"`
	Evaluator             string        `json:"evaluator"`
	ID                    string        `json:"id"`
	Remediation           string        `json:"remediation"`
	ReplacedBy            *string       `json:"replacedBy,omitempty"`
	SchemaVersion         SchemaVersion `json:"schemaVersion"`
	Severity              Severity      `json:"severity"`
	StandardReferences    []string      `json:"standardReferences,omitempty"`
	SupportedFhirVersions []FhirVersion `json:"supportedFhirVersions,omitempty"`
	TimeoutSeconds        int64         `json:"timeoutSeconds"`
	Title                 string        `json:"title"`
	Version               string        `json:"version"`
}

type RunManifest struct {
	CreatedAt      time.Time         `json:"createdAt"`
	FixtureVersion *string           `json:"fixtureVersion,omitempty"`
	ModelVersions  map[string]string `json:"modelVersions,omitempty"`
	OrganizationID string            `json:"organizationId"`
	Profile        string            `json:"profile"`
	ProjectID      string            `json:"projectId"`
	RuleVersions   map[string]string `json:"ruleVersions"`
	RunID          string            `json:"runId"`
	SchemaVersion  SchemaVersion     `json:"schemaVersion"`
	Target         TargetClass       `json:"target"`
}

type TargetClass struct {
	AllowPrivateNetwork *bool       `json:"allowPrivateNetwork,omitempty"`
	BaseURL             string      `json:"baseUrl"`
	CredentialRef       string      `json:"credentialRef"`
	FhirVersion         FhirVersion `json:"fhirVersion"`
	ID                  string      `json:"id"`
}

type Finding struct {
	EvidenceRefs  []string      `json:"evidenceRefs"`
	FindingID     string        `json:"findingId"`
	ObservedAt    time.Time     `json:"observedAt"`
	Outcome       Outcome       `json:"outcome"`
	Remediation   string        `json:"remediation"`
	RuleID        string        `json:"ruleId"`
	RuleVersion   string        `json:"ruleVersion"`
	RunID         string        `json:"runId"`
	SchemaVersion SchemaVersion `json:"schemaVersion"`
	Severity      Severity      `json:"severity"`
	Summary       string        `json:"summary"`
	Title         string        `json:"title"`
}

type Evidence struct {
	CreatedAt       time.Time       `json:"createdAt"`
	EvidenceID      string          `json:"evidenceId"`
	MediaType       string          `json:"mediaType"`
	RedactionStatus RedactionStatus `json:"redactionStatus"`
	RunID           string          `json:"runId"`
	SchemaVersion   SchemaVersion   `json:"schemaVersion"`
	Sha256          string          `json:"sha256"`
	SizeBytes       int64           `json:"sizeBytes"`
	SourceRuleID    *string         `json:"sourceRuleId,omitempty"`
	StorageURI      string          `json:"storageUri"`
}

type Report struct {
	Coverage       Coverage       `json:"coverage"`
	CreatedAt      time.Time      `json:"createdAt"`
	Decision       Decision       `json:"decision"`
	Findings       []ReportSchema `json:"findings"`
	ManifestSha256 string         `json:"manifestSha256"`
	ReportID       string         `json:"reportId"`
	RunID          string         `json:"runId"`
	SchemaVersion  SchemaVersion  `json:"schemaVersion"`
	Signature      *Signature     `json:"signature,omitempty"`
}

type Coverage struct {
	Completed int64 `json:"completed"`
	Selected  int64 `json:"selected"`
}

type ReportSchema struct {
	EvidenceRefs  []string      `json:"evidenceRefs"`
	FindingID     string        `json:"findingId"`
	ObservedAt    time.Time     `json:"observedAt"`
	Outcome       Outcome       `json:"outcome"`
	Remediation   string        `json:"remediation"`
	RuleID        string        `json:"ruleId"`
	RuleVersion   string        `json:"ruleVersion"`
	RunID         string        `json:"runId"`
	SchemaVersion SchemaVersion `json:"schemaVersion"`
	Severity      Severity      `json:"severity"`
	Summary       string        `json:"summary"`
	Title         string        `json:"title"`
}

type Signature struct {
	Algorithm Algorithm `json:"algorithm"`
	KeyID     string    `json:"keyId"`
	Value     string    `json:"value"`
}

type APIProblem struct {
	Category          APIProblemCategory `json:"category"`
	Code              string             `json:"code"`
	Detail            string             `json:"detail"`
	Instance          string             `json:"instance"`
	Retryable         bool               `json:"retryable"`
	RetryAfterSeconds *int64             `json:"retryAfterSeconds,omitempty"`
	RunID             *string            `json:"runId,omitempty"`
	SchemaVersion     SchemaVersion      `json:"schemaVersion"`
	Status            int64              `json:"status"`
	Title             string             `json:"title"`
	TraceID           string             `json:"traceId"`
	Type              string             `json:"type"`
}

type Behavior string

const (
	ActiveRead  Behavior = "active-read"
	ActiveWrite Behavior = "active-write"
	Passive     Behavior = "passive"
)

type Capability string

const (
	Fixtures          Capability = "fixtures"
	Model             Capability = "model"
	Network           Capability = "network"
	TargetCredentials Capability = "target-credentials"
	Write             Capability = "write"
)

type RuleCategory string

const (
	AISafety    RuleCategory = "ai-safety"
	Fhir        RuleCategory = "fhir"
	Reliability RuleCategory = "reliability"
	Security    RuleCategory = "security"
)

type SchemaVersion string

const (
	The100 SchemaVersion = "1.0.0"
)

type Severity string

const (
	Critical Severity = "critical"
	High     Severity = "high"
	Info     Severity = "info"
	Low      Severity = "low"
	Medium   Severity = "medium"
)

type FhirVersion string

const (
	The401 FhirVersion = "4.0.1"
)

type Outcome string

const (
	Fail          Outcome = "fail"
	Inconclusive  Outcome = "inconclusive"
	NotApplicable Outcome = "not_applicable"
	Pass          Outcome = "pass"
	PlatformError Outcome = "platform_error"
	Warning       Outcome = "warning"
)

type RedactionStatus string

const (
	NotRequired RedactionStatus = "not-required"
	Redacted    RedactionStatus = "redacted"
	Rejected    RedactionStatus = "rejected"
)

type Decision string

const (
	Conditional Decision = "conditional"
	Incomplete  Decision = "incomplete"
	NotReady    Decision = "not_ready"
	Ready       Decision = "ready"
)

type Algorithm string

const (
	Ed25519 Algorithm = "Ed25519"
)

type APIProblemCategory string

const (
	Authorization   APIProblemCategory = "authorization"
	Configuration   APIProblemCategory = "configuration"
	PermanentTarget APIProblemCategory = "permanent_target"
	Platform        APIProblemCategory = "platform"
	RuleDefect      APIProblemCategory = "rule_defect"
	Target          APIProblemCategory = "target"
	TransientTarget APIProblemCategory = "transient_target"
)
