import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ControlPlaneClient } from "./api-client";
import { createRepository } from "./repository";
import { flightcheckRepository } from "./demo-data";

// ---------------------------------------------------------------------------
// createRepository — env-var-driven factory
// ---------------------------------------------------------------------------

describe("createRepository()", () => {
  const originalEnv = { ...process.env };

  afterEach(() => {
    // Restore env between tests.
    for (const key of Object.keys(process.env)) {
      if (!(key in originalEnv)) delete process.env[key];
    }
    Object.assign(process.env, originalEnv);
  });

  it("returns the LocalDemoRepository when NEXT_PUBLIC_CONTROL_PLANE_URL is absent", () => {
    delete process.env.NEXT_PUBLIC_CONTROL_PLANE_URL;
    const repo = createRepository();
    // The demo repository is the exported singleton — same reference.
    expect(repo).toBe(flightcheckRepository);
  });

  it("returns a ControlPlaneClient when NEXT_PUBLIC_CONTROL_PLANE_URL is set", () => {
    process.env.NEXT_PUBLIC_CONTROL_PLANE_URL = "https://api.example.com";
    process.env.NEXT_PUBLIC_CONTROL_PLANE_TOKEN = "tok_test";
    process.env.NEXT_PUBLIC_CONTROL_PLANE_PROJECT_ID = "proj_1";
    process.env.NEXT_PUBLIC_CONTROL_PLANE_RUN_ID = "run_1";

    const repo = createRepository();
    expect(repo).toBeInstanceOf(ControlPlaneClient);
  });
});

// ---------------------------------------------------------------------------
// ControlPlaneClient — API mapping
// ---------------------------------------------------------------------------

describe("ControlPlaneClient.getOperationsSnapshot()", () => {
  const client = new ControlPlaneClient({
    baseUrl: "https://api.example.com",
    apiToken: "tok_test",
    projectId: "proj_abc",
    runId: "run_xyz",
  });

  const mockProject = {
    id: "proj_abc",
    name: "My FHIR project",
    organization: "Acme Health",
    profile: "startup-r4",
    policy: "Production gate",
  };

  const mockRun = {
    id: "run_xyz",
    state: "complete" as const,
    startedAt: "2026-08-18T23:29:00Z",
    completedAt: "2026-08-18T23:35:42Z",
    manifest: {
      target: {
        id: "target-1",
        name: "My sandbox",
        endpoint: "https://sandbox.example/fhir/R4",
        fhirVersion: "R4 · 4.0.1",
        environment: "Sandbox" as const,
      },
      ruleVersions: { "FHIR.001": "1.0", "SMART.001": "1.0" },
    },
    progress: {
      completed: 8,
      total: 10,
      packs: [
        { id: "fhir", name: "FHIR conformance", shortName: "FHIR", completed: 5, total: 5, state: "complete" as const },
        { id: "smart", name: "SMART App Launch", shortName: "SMRT", completed: 3, total: 5, state: "running" as const },
      ],
    },
  };

  const mockReport = {
    id: "rpt_001",
    runId: "run_xyz",
    evaluatedAt: "Aug 18, 2026 · 23:35 UTC",
    readiness: "not_ready" as const,
    decision: {
      title: "Release blocked",
      explanation: "Blocker found in SMART auth.",
    },
    findings: [
      {
        id: "f1",
        ruleId: "SMART.AUTH.001",
        title: "Missing scope",
        outcome: "fail" as const,
        severity: "blocker" as const,
        pack: "SMART",
        standard: "SMART App Launch 2.0",
        summary: "Required scope missing.",
        remediation: ["Add the missing scope."],
        evidenceIds: ["ev1"],
        regression: "new" as const,
      },
    ],
    evidence: [
      {
        id: "ev1",
        label: "Auth request",
        kind: "HTTP exchange" as const,
        capturedAt: "23:31:00 UTC",
        hash: "sha256:abc…def",
        redacted: true,
        summary: "Authorization request captured.",
        excerpt: ["GET /authorize"],
      },
    ],
  };

  const mockBaseline = {
    name: "release/2026.07",
    createdAt: "Jul 15, 2026",
    deltas: [
      { label: "Blockers", current: 1, baseline: 0, tone: "negative" as const },
    ],
  };

  beforeEach(() => {
    const responses: Record<string, unknown> = {
      "/healthz": { ok: true },
      "/v1/projects/proj_abc": mockProject,
      "/v1/runs/run_xyz": mockRun,
      "/v1/runs/run_xyz/report": mockReport,
      "/v1/projects/proj_abc/baseline": mockBaseline,
    };

    vi.stubGlobal(
      "fetch",
      vi.fn((url: string) => {
        const path = url.replace("https://api.example.com", "");
        const body = responses[path];
        if (!body) {
          return Promise.resolve({ ok: false, status: 404, statusText: "Not Found" });
        }
        return Promise.resolve({
          ok: true,
          json: () => Promise.resolve(body),
        });
      }),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("maps project name and organization", async () => {
    const snapshot = await client.getOperationsSnapshot();
    expect(snapshot.project.name).toBe("My FHIR project");
    expect(snapshot.project.organization).toBe("Acme Health");
  });

  it("maps the readiness decision", async () => {
    const snapshot = await client.getOperationsSnapshot();
    expect(snapshot.decision.readiness).toBe("not_ready");
    expect(snapshot.decision.title).toBe("Release blocked");
    expect(snapshot.decision.reportId).toBe("rpt_001");
  });

  it("maps findings with all required fields", async () => {
    const snapshot = await client.getOperationsSnapshot();
    expect(snapshot.findings).toHaveLength(1);
    const finding = snapshot.findings[0];
    expect(finding.ruleId).toBe("SMART.AUTH.001");
    expect(finding.severity).toBe("blocker");
    expect(finding.outcome).toBe("fail");
    expect(finding.regression).toBe("new");
    expect(finding.evidenceIds).toEqual(["ev1"]);
  });

  it("maps evidence artifacts", async () => {
    const snapshot = await client.getOperationsSnapshot();
    expect(snapshot.evidence).toHaveLength(1);
    expect(snapshot.evidence[0].kind).toBe("HTTP exchange");
    expect(snapshot.evidence[0].redacted).toBe(true);
  });

  it("maps packs from run progress", async () => {
    const snapshot = await client.getOperationsSnapshot();
    expect(snapshot.activeRun.packs).toHaveLength(2);
    expect(snapshot.activeRun.packs[0].id).toBe("fhir");
    expect(snapshot.activeRun.packs[1].state).toBe("running");
  });

  it("maps baseline deltas", async () => {
    const snapshot = await client.getOperationsSnapshot();
    expect(snapshot.baseline.name).toBe("release/2026.07");
    expect(snapshot.baseline.deltas[0].tone).toBe("negative");
  });

  it("maps run progress totals", async () => {
    const snapshot = await client.getOperationsSnapshot();
    expect(snapshot.activeRun.completed).toBe(8);
    expect(snapshot.activeRun.total).toBe(10);
  });

  it("throws when healthz fails", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(() =>
        Promise.resolve({ ok: false, status: 503, statusText: "Service Unavailable" }),
      ),
    );
    await expect(client.getOperationsSnapshot()).rejects.toThrow("503");
  });
});
