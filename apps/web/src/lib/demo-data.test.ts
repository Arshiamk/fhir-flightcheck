import { describe, expect, it } from "vitest";
import { demoSnapshot, flightcheckRepository } from "./demo-data";

describe("local demo repository", () => {
  it("keeps blocker findings aligned with the readiness decision", () => {
    const blockers = demoSnapshot.findings.filter(
      (finding) => finding.severity === "blocker" && finding.outcome === "fail",
    );

    expect(demoSnapshot.decision.readiness).toBe("not_ready");
    expect(blockers).toHaveLength(2);
    expect(demoSnapshot.metrics[0].value).toBe(String(blockers.length));
  });

  it("returns a copy that callers cannot use to mutate the source", async () => {
    const snapshot = await flightcheckRepository.getOperationsSnapshot();
    snapshot.project.name = "Changed by a consumer";

    const freshSnapshot = await flightcheckRepository.getOperationsSnapshot();
    expect(freshSnapshot.project.name).toBe("Atlas launch");
  });

  it("links every finding to existing evidence", () => {
    const evidenceIds = new Set(demoSnapshot.evidence.map(({ id }) => id));
    const missingIds = demoSnapshot.findings.flatMap(({ evidenceIds: linked }) =>
      linked.filter((id) => !evidenceIds.has(id)),
    );

    expect(missingIds).toEqual([]);
  });
});
