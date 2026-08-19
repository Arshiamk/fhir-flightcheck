package controlplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"
)

const (
	capabilityRuleID      = "fhir.capability.statement"
	capabilityRuleVersion = "1.0.0"
	maxCapabilityBytes    = 1 << 20
)

type CapabilityChecker struct {
	Policy  URLPolicy
	Timeout time.Duration
	Now     func() time.Time
}

type capabilityStatement struct {
	ResourceType string `json:"resourceType"`
	FHIRVersion  string `json:"fhirVersion"`
}

func (c CapabilityChecker) Check(ctx context.Context, run Run, target Target) CheckResult {
	now := c.now()
	finding := Finding{
		SchemaVersion: SchemaVersion, FindingID: newID("finding"), RunID: run.ID,
		RuleID: capabilityRuleID, RuleVersion: capabilityRuleVersion, Severity: "high",
		Title: "FHIR CapabilityStatement is valid", EvidenceRefs: []string{},
		Remediation: "Expose a FHIR R4 CapabilityStatement from the metadata endpoint.",
		ObservedAt:  now,
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	metadataURL := strings.TrimRight(target.BaseURL, "/") + "/metadata"
	request, err := http.NewRequestWithContext(checkCtx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return c.result(run.ID, finding, "platform_error", "Flightcheck could not build the metadata request.", nil, now)
	}
	request.Header.Set("Accept", "application/fhir+json, application/json")
	response, err := c.Policy.HTTPClient(target.AllowPrivateNetwork, timeout).Do(request)
	if err != nil {
		return c.result(run.ID, finding, "inconclusive", "The metadata endpoint could not be reached within the check deadline.", map[string]any{"error": Redact(err.Error())}, now)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxCapabilityBytes+1))
	summary := map[string]any{
		"httpStatus":  response.StatusCode,
		"contentType": response.Header.Get("Content-Type"),
		"bytesRead":   len(body),
	}
	if readErr != nil {
		summary["error"] = Redact(readErr.Error())
		return c.result(run.ID, finding, "inconclusive", "The metadata response could not be read.", summary, now)
	}
	if len(body) > maxCapabilityBytes {
		return c.result(run.ID, finding, "fail", "The metadata response exceeded the 1 MiB safety limit.", summary, now)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return c.result(run.ID, finding, "fail", fmt.Sprintf("The metadata endpoint returned HTTP %d.", response.StatusCode), summary, now)
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType != "application/fhir+json" && mediaType != "application/json" {
		return c.result(run.ID, finding, "fail", "The metadata endpoint did not return a supported FHIR JSON media type.", summary, now)
	}
	var statement capabilityStatement
	if err := json.Unmarshal(body, &statement); err != nil {
		return c.result(run.ID, finding, "fail", "The metadata response was not valid JSON.", summary, now)
	}
	summary["resourceType"] = statement.ResourceType
	summary["fhirVersion"] = statement.FHIRVersion
	if statement.ResourceType != "CapabilityStatement" {
		return c.result(run.ID, finding, "fail", "The metadata response was not a CapabilityStatement.", summary, now)
	}
	if statement.FHIRVersion != "4.0.1" {
		return c.result(run.ID, finding, "fail", "The CapabilityStatement did not declare FHIR R4 version 4.0.1.", summary, now)
	}
	return c.result(run.ID, finding, "pass", "The endpoint returned a FHIR R4 CapabilityStatement.", summary, now)
}

func (c CapabilityChecker) result(runID string, finding Finding, outcome, summary string, detail map[string]any, now time.Time) CheckResult {
	if detail == nil {
		detail = map[string]any{}
	}
	body, _ := json.Marshal(detail)
	hash := sha256.Sum256(body)
	evidenceID := newID("evidence")
	finding.Outcome = outcome
	finding.Summary = summary
	finding.EvidenceRefs = []string{evidenceID}
	return CheckResult{
		Finding: finding,
		Evidence: Evidence{
			SchemaVersion: SchemaVersion, EvidenceID: evidenceID, RunID: runID,
			MediaType: "application/json", SHA256: hex.EncodeToString(hash[:]),
			SizeBytes: int64(len(body)), StorageURI: "urn:sha256:" + hex.EncodeToString(hash[:]),
			RedactionStatus: "not-required", SourceRuleID: capabilityRuleID, CreatedAt: now,
		},
	}
}

func (c CapabilityChecker) now() time.Time {
	if c.Now != nil {
		return c.Now().UTC()
	}
	return time.Now().UTC()
}
