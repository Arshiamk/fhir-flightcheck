export type Readiness = "ready" | "conditional" | "not_ready";
export type Outcome =
  | "pass"
  | "fail"
  | "warning"
  | "inconclusive"
  | "not_applicable"
  | "platform_error";
export type RunState = "complete" | "running" | "queued";

export interface Metric {
  label: string;
  value: string;
  detail: string;
}

export interface Target {
  id: string;
  name: string;
  endpoint: string;
  environment: "Synthetic" | "Sandbox";
  fhirVersion: string;
  readiness: Readiness;
  blockers: number;
  lastChecked: string;
  latency: string;
}

export interface PackProgress {
  id: string;
  name: string;
  shortName: string;
  complete: number;
  total: number;
  state: RunState;
}

export interface Evidence {
  id: string;
  label: string;
  kind: "HTTP exchange" | "FHIR resource" | "Trace" | "Policy evaluation";
  capturedAt: string;
  hash: string;
  redacted: boolean;
  summary: string;
  excerpt: string[];
}

export interface Finding {
  id: string;
  ruleId: string;
  title: string;
  outcome: Outcome;
  severity: "blocker" | "high" | "medium" | "low";
  pack: string;
  standard: string;
  summary: string;
  remediation: string[];
  evidenceIds: string[];
  regression: "new" | "existing" | "resolved";
}

export interface BaselineDelta {
  label: string;
  current: number;
  baseline: number;
  tone: "positive" | "negative" | "neutral";
}

export interface AuditEvent {
  id: string;
  actor: string;
  action: string;
  resource: string;
  timestamp: string;
}

export interface OperationsSnapshot {
  project: {
    name: string;
    organization: string;
    profile: string;
    policy: string;
  };
  decision: {
    readiness: Readiness;
    title: string;
    explanation: string;
    evaluatedAt: string;
    reportId: string;
  };
  metrics: Metric[];
  targets: Target[];
  activeRun: {
    id: string;
    targetName: string;
    startedAt: string;
    elapsed: string;
    completed: number;
    total: number;
    packs: PackProgress[];
  };
  findings: Finding[];
  evidence: Evidence[];
  baseline: {
    name: string;
    createdAt: string;
    deltas: BaselineDelta[];
  };
  policy: {
    name: string;
    version: string;
    blockerRule: string;
    warningBudget: string;
    evidenceMinimum: string;
  };
  audit: AuditEvent[];
}

export interface FlightcheckRepository {
  getOperationsSnapshot(): Promise<OperationsSnapshot>;
}
