import type { FlightcheckRepository, OperationsSnapshot } from "./types";

export const demoSnapshot: OperationsSnapshot = {
  project: {
    name: "Atlas launch",
    organization: "Northstar Health",
    profile: "startup-r4",
    policy: "Production gate",
  },
  decision: {
    readiness: "not_ready",
    title: "Release blocked",
    explanation:
      "Two policy blockers must be resolved before this target is ready for production traffic.",
    evaluatedAt: "Aug 18, 2026 · 23:36 UTC",
    reportId: "rpt_01J5P8XK7Q",
  },
  metrics: [
    { label: "Policy blockers", value: "2", detail: "2 new since baseline" },
    { label: "Evidence coverage", value: "94%", detail: "47 of 50 checks" },
    { label: "Checks passed", value: "41", detail: "82% deterministic pass" },
    { label: "Run duration", value: "06:42", detail: "1m 18s faster" },
  ],
  targets: [
    {
      id: "target-sandbox",
      name: "Atlas EHR sandbox",
      endpoint: "https://sandbox.atlas.example/fhir/R4",
      environment: "Sandbox",
      fhirVersion: "R4 · 4.0.1",
      readiness: "not_ready",
      blockers: 2,
      lastChecked: "4 min ago",
      latency: "184 ms",
    },
    {
      id: "target-synthetic",
      name: "Synthea reference",
      endpoint: "local://synthea-r4",
      environment: "Synthetic",
      fhirVersion: "R4 · 4.0.1",
      readiness: "ready",
      blockers: 0,
      lastChecked: "18 min ago",
      latency: "31 ms",
    },
    {
      id: "target-recovery",
      name: "Recovery scenario",
      endpoint: "local://fault-injection",
      environment: "Synthetic",
      fhirVersion: "R4 · 4.0.1",
      readiness: "conditional",
      blockers: 0,
      lastChecked: "1 hr ago",
      latency: "412 ms",
    },
  ],
  activeRun: {
    id: "run_01J5P8S4MA",
    targetName: "Atlas EHR sandbox",
    startedAt: "23:29 UTC",
    elapsed: "06:42",
    completed: 47,
    total: 50,
    packs: [
      { id: "fhir", name: "FHIR conformance & data quality", shortName: "FHIR", complete: 18, total: 18, state: "complete" },
      { id: "reliability", name: "Reliability & recovery", shortName: "RELY", complete: 11, total: 11, state: "complete" },
      { id: "security", name: "Security, privacy & auditability", shortName: "SECR", complete: 12, total: 13, state: "running" },
      { id: "ai", name: "AI safety & human oversight", shortName: "SAFE", complete: 6, total: 8, state: "running" },
    ],
  },
  findings: [
    {
      id: "finding-scope",
      ruleId: "SMART.AUTH.004",
      title: "Patient write scope exceeds launch context",
      outcome: "fail",
      severity: "blocker",
      pack: "Security & privacy",
      standard: "SMART App Launch 2.2",
      summary:
        "The authorization request includes patient/*.write although this workflow is configured as read-only.",
      remediation: [
        "Replace patient/*.write with the resource-specific read scopes required by the workflow.",
        "Invalidate the sandbox client secret and complete a fresh authorization launch.",
        "Re-run the Production gate profile and attach the resulting SMART discovery evidence.",
      ],
      evidenceIds: ["ev-auth-request", "ev-policy"],
      regression: "new",
    },
    {
      id: "finding-ai-write",
      ruleId: "AI.TOOL.007",
      title: "AI workflow attempted an unapproved clinical write",
      outcome: "fail",
      severity: "blocker",
      pack: "AI safety",
      standard: "Flightcheck AI controls v1.3",
      summary:
        "The synthetic discharge workflow invoked MedicationRequest.create without a human approval grant.",
      remediation: [
        "Remove create capability from the model tool manifest.",
        "Route medication proposals to the configured human-review queue.",
        "Verify the denial path with case golden-ai-014 before promoting.",
      ],
      evidenceIds: ["ev-tool-trace", "ev-policy"],
      regression: "new",
    },
    {
      id: "finding-pagination",
      ruleId: "FHIR.SEARCH.012",
      title: "Duplicate page detected during Patient search",
      outcome: "warning",
      severity: "medium",
      pack: "FHIR conformance",
      standard: "FHIR R4 Bundle",
      summary:
        "The second page repeated three resources from the first page after a transient 429 response.",
      remediation: [
        "Persist and resume the server-provided next link rather than rebuilding the query.",
        "Deduplicate resources by resourceType/id before downstream processing.",
      ],
      evidenceIds: ["ev-pagination"],
      regression: "existing",
    },
    {
      id: "finding-audit",
      ruleId: "AUDIT.TRACE.003",
      title: "Correlation identifier missing from one audit event",
      outcome: "inconclusive",
      severity: "low",
      pack: "Security & privacy",
      standard: "FHIR R4 AuditEvent",
      summary:
        "A token refresh event was observed, but its trace identifier was not present in the exported audit record.",
      remediation: [
        "Propagate the W3C traceparent into AuditEvent.entity.detail.",
        "Repeat the refresh-token scenario with audit export enabled.",
      ],
      evidenceIds: ["ev-audit"],
      regression: "existing",
    },
  ],
  evidence: [
    {
      id: "ev-auth-request",
      label: "SMART authorization request",
      kind: "HTTP exchange",
      capturedAt: "23:31:08 UTC",
      hash: "sha256:14ac…a992",
      redacted: true,
      summary: "Authorization parameters captured before redirect.",
      excerpt: [
        "GET /authorize?aud=https://sandbox.atlas.example/fhir/R4",
        "scope=openid+fhirUser+patient/*.read+patient/*.write",
        "client_id=[REDACTED]  launch=[REDACTED]",
        "HTTP/2 302 · location validated",
      ],
    },
    {
      id: "ev-policy",
      label: "Production gate evaluation",
      kind: "Policy evaluation",
      capturedAt: "23:35:51 UTC",
      hash: "sha256:884f…2d10",
      redacted: false,
      summary: "Decision trace for blocker and evidence policies.",
      excerpt: [
        "policy: production-gate@3.2.0",
        "deny if severity == blocker && outcome == fail",
        "matched: SMART.AUTH.004, AI.TOOL.007",
        "decision: NOT_READY",
      ],
    },
    {
      id: "ev-tool-trace",
      label: "Guardrailed tool trace",
      kind: "Trace",
      capturedAt: "23:34:27 UTC",
      hash: "sha256:7bc1…f420",
      redacted: true,
      summary: "Synthetic model/tool exchange with denied write.",
      excerpt: [
        "case: golden-ai-014 · model: deterministic-stub@1.4",
        "requested_tool: MedicationRequest.create",
        "capability_grant: read:fhir/*",
        "result: DENIED · reason: HUMAN_APPROVAL_REQUIRED",
      ],
    },
    {
      id: "ev-pagination",
      label: "Patient search page sequence",
      kind: "FHIR resource",
      capturedAt: "23:32:19 UTC",
      hash: "sha256:5f07…1e44",
      redacted: true,
      summary: "Synthetic Bundle identifiers and pagination links.",
      excerpt: [
        "Bundle.type: searchset · page: 2",
        "entry count: 20 · repeated ids: 3",
        "retry-after observed: 2s",
        "payload: synthetic · direct identifiers removed",
      ],
    },
    {
      id: "ev-audit",
      label: "Refresh token audit export",
      kind: "FHIR resource",
      capturedAt: "23:33:42 UTC",
      hash: "sha256:c20e…8a11",
      redacted: true,
      summary: "AuditEvent generated for a synthetic refresh flow.",
      excerpt: [
        "AuditEvent.action: E · outcome: 0",
        "agent.requestor: true · identity: [REDACTED]",
        "trace_id: null",
        "classification: synthetic / no PHI",
      ],
    },
  ],
  baseline: {
    name: "release/2026.08",
    createdAt: "Aug 11, 2026",
    deltas: [
      { label: "Blockers", current: 2, baseline: 0, tone: "negative" },
      { label: "Warnings", current: 1, baseline: 2, tone: "positive" },
      { label: "Passed", current: 41, baseline: 39, tone: "positive" },
      { label: "Coverage", current: 94, baseline: 96, tone: "negative" },
    ],
  },
  policy: {
    name: "Production gate",
    version: "3.2.0",
    blockerRule: "No failed blocker findings",
    warningBudget: "≤ 3 unresolved warnings",
    evidenceMinimum: "≥ 95% deterministic coverage",
  },
  audit: [
    { id: "aud-1", actor: "Maya Chen", action: "started run", resource: "run_01J5P8S4MA", timestamp: "23:29:09" },
    { id: "aud-2", actor: "flightcheck-worker", action: "redacted evidence", resource: "ev-auth-request", timestamp: "23:31:08" },
    { id: "aud-3", actor: "policy-engine", action: "evaluated gate", resource: "production-gate@3.2.0", timestamp: "23:35:51" },
    { id: "aud-4", actor: "Maya Chen", action: "viewed report", resource: "rpt_01J5P8XK7Q", timestamp: "23:38:12" },
  ],
};

class LocalDemoRepository implements FlightcheckRepository {
  async getOperationsSnapshot(): Promise<OperationsSnapshot> {
    return structuredClone(demoSnapshot);
  }
}

export const flightcheckRepository: FlightcheckRepository =
  new LocalDemoRepository();
