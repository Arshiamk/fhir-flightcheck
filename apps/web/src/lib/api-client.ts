import type {
  AuditEvent,
  BaselineDelta,
  Evidence,
  Finding,
  FlightcheckRepository,
  Metric,
  OperationsSnapshot,
  Outcome,
  PackProgress,
  Readiness,
  Target,
} from "./types";

// Minimal shapes for the control-plane API responses we care about.
interface ApiProject {
  id: string;
  name: string;
  organization?: string;
  profile?: string;
  policy?: string;
}

interface ApiRun {
  id: string;
  state: "complete" | "running" | "queued";
  startedAt?: string;
  completedAt?: string;
  manifest?: {
    target?: {
      id?: string;
      name?: string;
      endpoint?: string;
      fhirVersion?: string;
      environment?: "Synthetic" | "Sandbox";
    };
    ruleVersions?: Record<string, string>;
  };
  progress?: {
    completed: number;
    total: number;
    packs?: Array<{
      id: string;
      name: string;
      shortName?: string;
      completed: number;
      total: number;
      state: "complete" | "running" | "queued";
    }>;
  };
}

interface ApiReport {
  id: string;
  runId: string;
  evaluatedAt?: string;
  readiness: Readiness;
  decision?: {
    title?: string;
    explanation?: string;
  };
  findings?: Array<{
    id: string;
    ruleId: string;
    title: string;
    outcome: Outcome;
    severity: "blocker" | "high" | "medium" | "low";
    pack?: string;
    standard?: string;
    summary?: string;
    remediation?: string[];
    evidenceIds?: string[];
    regression?: "new" | "existing" | "resolved";
  }>;
  evidence?: Array<{
    id: string;
    label: string;
    kind: "HTTP exchange" | "FHIR resource" | "Trace" | "Policy evaluation";
    capturedAt?: string;
    hash?: string;
    redacted?: boolean;
    summary?: string;
    excerpt?: string[];
  }>;
  metrics?: Array<{
    label: string;
    value: string;
    detail: string;
  }>;
}

interface ApiBaseline {
  name?: string;
  createdAt?: string;
  deltas?: Array<{
    label: string;
    current: number;
    baseline: number;
    tone: "positive" | "negative" | "neutral";
  }>;
}

export class ControlPlaneClient implements FlightcheckRepository {
  private readonly baseUrl: string;
  private readonly apiToken: string;
  private readonly projectId: string;
  private readonly runId: string;

  constructor(opts: {
    baseUrl: string;
    apiToken: string;
    projectId: string;
    runId: string;
  }) {
    this.baseUrl = opts.baseUrl.replace(/\/$/, "");
    this.apiToken = opts.apiToken;
    this.projectId = opts.projectId;
    this.runId = opts.runId;
  }

  private async fetch<T>(path: string): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const response = await fetch(url, {
      headers: {
        Authorization: `Bearer ${this.apiToken}`,
        "Content-Type": "application/json",
      },
      cache: "no-store",
    });
    if (!response.ok) {
      throw new Error(
        `Control-plane request failed: ${response.status} ${response.statusText} (${url})`,
      );
    }
    return response.json() as Promise<T>;
  }

  async getOperationsSnapshot(): Promise<OperationsSnapshot> {
    // Liveness check — throw early if the API is unreachable.
    await this.fetch<unknown>("/healthz");

    const [project, run, report, baseline] = await Promise.all([
      this.fetch<ApiProject>(`/v1/projects/${this.projectId}`),
      this.fetch<ApiRun>(`/v1/runs/${this.runId}`),
      this.fetch<ApiReport>(`/v1/runs/${this.runId}/report`),
      this.fetch<ApiBaseline>(`/v1/projects/${this.projectId}/baseline`).catch(
        (): ApiBaseline => ({}),
      ),
    ]);

    return mapToSnapshot({ project, run, report, baseline });
  }
}

// ---------------------------------------------------------------------------
// Mapping helpers
// ---------------------------------------------------------------------------

function mapToSnapshot({
  project,
  run,
  report,
  baseline,
}: {
  project: ApiProject;
  run: ApiRun;
  report: ApiReport;
  baseline: ApiBaseline;
}): OperationsSnapshot {
  const target = run.manifest?.target;
  const progress = run.progress ?? { completed: 0, total: 1 };
  const ruleVersions = run.manifest?.ruleVersions ?? {};

  const targets: Target[] = [
    {
      id: target?.id ?? run.id,
      name: target?.name ?? "Unknown target",
      endpoint: target?.endpoint ?? "",
      environment: target?.environment ?? "Sandbox",
      fhirVersion: target?.fhirVersion ?? "R4",
      readiness: report.readiness,
      blockers: countBlockers(report.findings ?? []),
      lastChecked: formatRelative(run.completedAt ?? run.startedAt),
      latency: "—",
    },
  ];

  const packs: PackProgress[] = buildPacks(progress.packs, ruleVersions);

  const findings: Finding[] = (report.findings ?? []).map((f) => ({
    id: f.id,
    ruleId: f.ruleId,
    title: f.title,
    outcome: f.outcome,
    severity: f.severity,
    pack: f.pack ?? "General",
    standard: f.standard ?? "",
    summary: f.summary ?? "",
    remediation: f.remediation ?? [],
    evidenceIds: f.evidenceIds ?? [],
    regression: f.regression ?? "existing",
  }));

  const evidence: Evidence[] = (report.evidence ?? []).map((e) => ({
    id: e.id,
    label: e.label,
    kind: e.kind,
    capturedAt: e.capturedAt ?? "",
    hash: e.hash ?? "",
    redacted: e.redacted ?? false,
    summary: e.summary ?? "",
    excerpt: e.excerpt ?? [],
  }));

  const metrics: Metric[] = report.metrics?.length
    ? report.metrics
    : synthesizeMetrics(report, progress);

  const blocker = findings.find(
    (f) => f.severity === "blocker" && f.outcome === "fail",
  );
  const readiness: Readiness = report.readiness;
  const decisionTitle = report.decision?.title ?? defaultDecisionTitle(readiness);
  const decisionExplanation =
    report.decision?.explanation ??
    (blocker ? blocker.summary : "No issues found.");

  const auditLog: AuditEvent[] = [];

  const baselineDeltas: BaselineDelta[] = baseline.deltas ?? [];

  return {
    project: {
      name: project.name,
      organization: project.organization ?? "",
      profile: project.profile ?? "",
      policy: project.policy ?? "",
    },
    decision: {
      readiness,
      title: decisionTitle,
      explanation: decisionExplanation,
      evaluatedAt: report.evaluatedAt ?? "",
      reportId: report.id,
    },
    metrics,
    targets,
    activeRun: {
      id: run.id,
      targetName: target?.name ?? "Unknown target",
      startedAt: formatTime(run.startedAt),
      elapsed: elapsedString(run.startedAt, run.completedAt),
      completed: progress.completed,
      total: progress.total,
      packs,
    },
    findings,
    evidence,
    baseline: {
      name: baseline.name ?? "—",
      createdAt: baseline.createdAt ?? "—",
      deltas: baselineDeltas,
    },
    policy: {
      name: project.policy ?? "—",
      version: "—",
      blockerRule: "No failed blocker findings",
      warningBudget: "—",
      evidenceMinimum: "—",
    },
    audit: auditLog,
  };
}

function countBlockers(
  findings: NonNullable<ApiReport["findings"]>,
): number {
  return findings.filter(
    (f) => f.severity === "blocker" && f.outcome === "fail",
  ).length;
}

type ApiPack = NonNullable<NonNullable<ApiRun["progress"]>["packs"]>[number];

function buildPacks(
  apiPacks: ApiPack[] | undefined,
  ruleVersions: Record<string, string>,
): PackProgress[] {
  if (apiPacks && apiPacks.length > 0) {
    return apiPacks.map((p) => ({
      id: p.id,
      name: p.name,
      shortName: p.shortName ?? p.id.toUpperCase().slice(0, 4),
      complete: p.completed,
      total: p.total,
      state: p.state,
    }));
  }

  // Synthesize one pack per category inferred from rule prefix counts.
  const categories: Record<string, number> = {};
  for (const key of Object.keys(ruleVersions)) {
    const prefix = key.split(".")[0] ?? "MISC";
    categories[prefix] = (categories[prefix] ?? 0) + 1;
  }

  if (Object.keys(categories).length === 0) {
    return [];
  }

  return Object.entries(categories).map(([prefix, count]) => ({
    id: prefix.toLowerCase(),
    name: prefix,
    shortName: prefix.slice(0, 4).toUpperCase(),
    complete: count,
    total: count,
    state: "complete" as const,
  }));
}

function synthesizeMetrics(
  report: ApiReport,
  progress: { completed: number; total: number },
): Metric[] {
  const blockers = countBlockers(report.findings ?? []);
  const total = progress.total;
  const completed = progress.completed;
  const pct = total > 0 ? Math.round((completed / total) * 100) : 0;
  const passed = (report.findings ?? []).filter((f) => f.outcome === "pass").length;

  return [
    { label: "Policy blockers", value: String(blockers), detail: `${blockers} found` },
    { label: "Evidence coverage", value: `${pct}%`, detail: `${completed} of ${total} checks` },
    { label: "Checks passed", value: String(passed), detail: `${total > 0 ? Math.round((passed / total) * 100) : 0}% pass rate` },
    { label: "Run state", value: report.readiness === "ready" ? "Ready" : "Blocked", detail: "" },
  ];
}

function defaultDecisionTitle(readiness: Readiness): string {
  if (readiness === "ready") return "Ready for release";
  if (readiness === "conditional") return "Conditional release";
  return "Release blocked";
}

function formatTime(isoString: string | undefined): string {
  if (!isoString) return "—";
  try {
    return new Date(isoString).toISOString().substring(11, 19) + " UTC";
  } catch {
    return isoString;
  }
}

function formatRelative(isoString: string | undefined): string {
  if (!isoString) return "—";
  try {
    const diffMs = Date.now() - new Date(isoString).getTime();
    const mins = Math.floor(diffMs / 60_000);
    if (mins < 60) return `${mins} min ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs} hr ago`;
    return `${Math.floor(hrs / 24)} days ago`;
  } catch {
    return "—";
  }
}

function elapsedString(
  startedAt: string | undefined,
  completedAt: string | undefined,
): string {
  if (!startedAt) return "—";
  try {
    const end = completedAt ? new Date(completedAt) : new Date();
    const diffMs = end.getTime() - new Date(startedAt).getTime();
    const totalSecs = Math.floor(diffMs / 1000);
    const mins = String(Math.floor(totalSecs / 60)).padStart(2, "0");
    const secs = String(totalSecs % 60).padStart(2, "0");
    return `${mins}:${secs}`;
  } catch {
    return "—";
  }
}
