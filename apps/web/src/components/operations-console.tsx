"use client";

import { useMemo, useState } from "react";
import type {
  Evidence,
  Finding,
  OperationsSnapshot,
  Outcome,
  Readiness,
} from "@/lib/types";
import {
  ArrowUpIcon,
  AuditIcon,
  ChevronIcon,
  DashboardIcon,
  EvidenceIcon,
  LockIcon,
  PulseIcon,
  RunsIcon,
  SearchIcon,
  SettingsIcon,
  ShieldIcon,
  TargetIcon,
} from "./icons";

const outcomeLabels: Record<Outcome, string> = {
  pass: "Passed",
  fail: "Failed",
  warning: "Warning",
  inconclusive: "Inconclusive",
  not_applicable: "Not applicable",
  platform_error: "Platform error",
};

const readinessLabels: Record<Readiness, string> = {
  ready: "Ready",
  conditional: "Conditional",
  not_ready: "Not ready",
};

const navItems = [
  { label: "Overview", href: "#overview", icon: DashboardIcon },
  { label: "Targets", href: "#targets", icon: TargetIcon },
  { label: "Runs", href: "#active-run", icon: RunsIcon },
  { label: "Findings", href: "#findings", icon: ShieldIcon, count: 2 },
  { label: "Evidence", href: "#evidence", icon: EvidenceIcon },
  { label: "Policies", href: "#policy", icon: SettingsIcon },
  { label: "Audit log", href: "#audit", icon: AuditIcon },
];

function StatusMark({
  status,
}: {
  status: Readiness | Outcome | "blocker" | "running" | "complete";
}) {
  const symbol =
    status === "ready" || status === "pass" || status === "complete"
      ? "✓"
      : status === "warning" || status === "conditional" || status === "inconclusive"
        ? "!"
        : status === "running"
          ? "↻"
          : "×";
  return (
    <span aria-hidden="true" className={`status-mark status-${status}`}>
      {symbol}
    </span>
  );
}

function ReadinessBadge({ value }: { value: Readiness }) {
  return (
    <span className={`readiness-badge badge-${value}`}>
      <StatusMark status={value} />
      {readinessLabels[value]}
    </span>
  );
}

function FindingCard({
  finding,
  selected,
  onSelect,
}: {
  finding: Finding;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      aria-pressed={selected}
      className={`finding-card ${selected ? "is-selected" : ""}`}
      onClick={onSelect}
      type="button"
    >
      <span className="finding-leading">
        <StatusMark status={finding.severity === "blocker" ? "blocker" : finding.outcome} />
        <span>
          <span className="finding-meta">
            <span className={`severity severity-${finding.severity}`}>
              {finding.severity}
            </span>
            <span>{finding.ruleId}</span>
            {finding.regression === "new" && <span className="new-flag">New</span>}
          </span>
          <strong>{finding.title}</strong>
          <span className="finding-summary">{finding.summary}</span>
        </span>
      </span>
      <ChevronIcon className="chevron" />
    </button>
  );
}

function EvidenceDetail({ evidence }: { evidence: Evidence }) {
  return (
    <article className="evidence-detail" aria-labelledby="evidence-detail-title">
      <div className="detail-header">
        <div>
          <span className="eyebrow">{evidence.kind}</span>
          <h3 id="evidence-detail-title">{evidence.label}</h3>
        </div>
        {evidence.redacted && (
          <span className="redacted-badge">
            <LockIcon /> Redacted
          </span>
        )}
      </div>
      <p className="detail-summary">{evidence.summary}</p>
      <dl className="evidence-meta">
        <div>
          <dt>Captured</dt>
          <dd>{evidence.capturedAt}</dd>
        </div>
        <div>
          <dt>Content hash</dt>
          <dd>{evidence.hash}</dd>
        </div>
      </dl>
      <pre aria-label={`${evidence.label} excerpt`}>
        {evidence.excerpt.map((line, index) => (
          <code key={line}>
            <span aria-hidden="true">{String(index + 1).padStart(2, "0")}</span>
            {line}
            {"\n"}
          </code>
        ))}
      </pre>
      <div className="provenance-note">
        <ShieldIcon />
        <span>
          Evidence integrity verified. Synthetic inputs only; direct identifiers
          removed before storage.
        </span>
      </div>
    </article>
  );
}

export function OperationsConsole({
  data,
  isLive = false,
}: {
  data: OperationsSnapshot;
  isLive?: boolean;
}) {
  const [selectedFindingId, setSelectedFindingId] = useState(data.findings[0].id);
  const [selectedEvidenceId, setSelectedEvidenceId] = useState(
    data.findings[0].evidenceIds[0],
  );
  const [findingFilter, setFindingFilter] = useState<"all" | "blocker" | "warning">(
    "all",
  );
  const [query, setQuery] = useState("");

  const selectedFinding =
    data.findings.find((finding) => finding.id === selectedFindingId) ??
    data.findings[0];
  const selectedEvidence =
    data.evidence.find((evidence) => evidence.id === selectedEvidenceId) ??
    data.evidence[0];
  const filteredFindings = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return data.findings.filter((finding) => {
      const matchesFilter =
        findingFilter === "all" ||
        finding.severity === findingFilter ||
        finding.outcome === findingFilter;
      const matchesQuery =
        !normalizedQuery ||
        `${finding.title} ${finding.ruleId} ${finding.pack}`
          .toLowerCase()
          .includes(normalizedQuery);
      return matchesFilter && matchesQuery;
    });
  }, [data.findings, findingFilter, query]);

  function selectFinding(finding: Finding) {
    setSelectedFindingId(finding.id);
    setSelectedEvidenceId(finding.evidenceIds[0]);
  }

  const runPercent = Math.round(
    (data.activeRun.completed / data.activeRun.total) * 100,
  );

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">
        Skip to main content
      </a>
      <aside className="sidebar" aria-label="Primary navigation">
        <a className="brand" href="#overview" aria-label="FHIR Flightcheck home">
          <span className="brand-mark"><PulseIcon /></span>
          <span>
            <strong>FHIR</strong>
            <span>FLIGHTCHECK</span>
          </span>
        </a>
        <nav>
          <p className="nav-label">OPERATIONS</p>
          {navItems.map(({ label, href, icon: NavIcon, count }, index) => (
            <a aria-label={label} className={index === 0 ? "active" : ""} href={href} key={href}>
              <NavIcon />
              <span>{label}</span>
              {count && <span className="nav-count" aria-label={`${count} blockers`}>{count}</span>}
            </a>
          ))}
        </nav>
        <div className="sidebar-safety">
          <ShieldIcon />
          <div>
            <strong>Synthetic mode</strong>
            <span>No production PHI</span>
          </div>
        </div>
        <div className="user">
          <span className="avatar" aria-hidden="true">MC</span>
          <span>
            <strong>Maya Chen</strong>
            <span>Platform lead</span>
          </span>
          <button aria-label="Open account menu" type="button">•••</button>
        </div>
      </aside>

      <main id="main-content" tabIndex={-1}>
        <header className="topbar">
          <div className="breadcrumbs" aria-label="Breadcrumb">
            <span>{data.project.organization}</span>
            <span aria-hidden="true">/</span>
            <strong>{data.project.name}</strong>
          </div>
          <div className="topbar-actions">
            {isLive ? (
              <span className="live-indicator live-indicator--live">
                <span aria-hidden="true" className="live-dot" />
                Live
              </span>
            ) : (
              <span className="live-indicator"><span aria-hidden="true" /> Demo data</span>
            )}
            <button className="ghost-button" type="button">Export report</button>
            <button className="primary-button" type="button">
              <PulseIcon /> New run
            </button>
          </div>
        </header>

        <div className="content">
          <section className="hero" id="overview" aria-labelledby="page-title">
            <div>
              <p className="eyebrow">PRODUCTION READINESS / CURRENT REPORT</p>
              <h1 id="page-title">Operations console</h1>
              <p className="hero-copy">
                Evidence-backed readiness for your FHIR R4 integration.
              </p>
            </div>
            <div className="report-meta">
              <span>Report</span>
              <strong>{data.decision.reportId}</strong>
              <span>{data.decision.evaluatedAt}</span>
            </div>
          </section>

          <section className="decision-grid" aria-label="Readiness summary">
            <article className="decision-card">
              <div className="decision-signal" aria-hidden="true">
                <span className="signal-ring"><span>×</span></span>
              </div>
              <div className="decision-copy">
                <p className="eyebrow">POLICY DECISION</p>
                <h2>{data.decision.title}</h2>
                <p>{data.decision.explanation}</p>
                <div className="decision-footer">
                  <ReadinessBadge value={data.decision.readiness} />
                  <a href="#findings">Review blockers <ChevronIcon /></a>
                </div>
              </div>
            </article>
            <div className="metrics-grid">
              {data.metrics.map((metric, index) => (
                <article className="metric-card" key={metric.label}>
                  <span>{metric.label}</span>
                  <strong>{metric.value}</strong>
                  <small className={index === 0 ? "negative" : ""}>{metric.detail}</small>
                </article>
              ))}
            </div>
          </section>

          <section className="section" id="targets" aria-labelledby="targets-title">
            <div className="section-heading">
              <div>
                <p className="eyebrow">CONNECTED ENVIRONMENTS</p>
                <h2 id="targets-title">Targets</h2>
              </div>
              <button className="text-button" type="button">Manage targets <ChevronIcon /></button>
            </div>
            <div className="target-grid">
              {data.targets.map((target) => (
                <article className="target-card" key={target.id}>
                  <div className="target-top">
                    <span className="target-icon"><TargetIcon /></span>
                    <ReadinessBadge value={target.readiness} />
                  </div>
                  <h3>{target.name}</h3>
                  <code>{target.endpoint}</code>
                  <div className="target-tags">
                    <span>{target.environment}</span>
                    <span>{target.fhirVersion}</span>
                  </div>
                  <dl>
                    <div><dt>Last checked</dt><dd>{target.lastChecked}</dd></div>
                    <div><dt>Median latency</dt><dd>{target.latency}</dd></div>
                    <div><dt>Blockers</dt><dd>{target.blockers}</dd></div>
                  </dl>
                </article>
              ))}
            </div>
          </section>

          <section className="section" id="active-run" aria-labelledby="run-title">
            <div className="section-heading">
              <div>
                <p className="eyebrow">LIVE EXECUTION</p>
                <h2 id="run-title">Run progress</h2>
              </div>
              <span className="run-id">{data.activeRun.id}</span>
            </div>
            <article className="run-panel">
              <div className="run-overview">
                <div>
                  <span className="running-badge"><StatusMark status="running" /> Running</span>
                  <h3>{data.activeRun.targetName}</h3>
                  <p>Started {data.activeRun.startedAt} · elapsed {data.activeRun.elapsed}</p>
                </div>
                <div className="run-total" aria-label={`${runPercent} percent complete`}>
                  <strong>{runPercent}%</strong>
                  <span>{data.activeRun.completed}/{data.activeRun.total} checks</span>
                </div>
              </div>
              <div className="overall-progress" role="progressbar" aria-label="Overall run progress" aria-valuemin={0} aria-valuemax={100} aria-valuenow={runPercent}>
                <span style={{ width: `${runPercent}%` }} />
              </div>
              <div className="pack-grid">
                {data.activeRun.packs.map((pack) => {
                  const percentage = Math.round((pack.complete / pack.total) * 100);
                  return (
                    <div className="pack-progress" key={pack.id}>
                      <div>
                        <span className="pack-code">{pack.shortName}</span>
                        <span>
                          <strong>{pack.name}</strong>
                          <small>{pack.complete} of {pack.total} checks</small>
                        </span>
                        <StatusMark status={pack.state === "complete" ? "complete" : "running"} />
                      </div>
                      <div className="slim-progress" role="progressbar" aria-label={`${pack.name} progress`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={percentage}>
                        <span style={{ width: `${percentage}%` }} />
                      </div>
                    </div>
                  );
                })}
              </div>
            </article>
          </section>

          <section className="section" id="findings" aria-labelledby="findings-title">
            <div className="section-heading findings-heading">
              <div>
                <p className="eyebrow">BLOCKER-FIRST REVIEW</p>
                <h2 id="findings-title">Findings</h2>
              </div>
              <div className="findings-controls">
                <label className="search-field">
                  <span className="sr-only">Search findings</span>
                  <SearchIcon />
                  <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search findings" />
                </label>
                <div className="filter-group" aria-label="Filter findings" role="group">
                  {(["all", "blocker", "warning"] as const).map((filter) => (
                    <button aria-pressed={findingFilter === filter} key={filter} onClick={() => setFindingFilter(filter)} type="button">
                      {filter}
                    </button>
                  ))}
                </div>
              </div>
            </div>
            <div className="findings-layout">
              <div className="finding-list" aria-live="polite">
                {filteredFindings.length ? (
                  filteredFindings.map((finding) => (
                    <FindingCard finding={finding} key={finding.id} onSelect={() => selectFinding(finding)} selected={finding.id === selectedFinding.id} />
                  ))
                ) : (
                  <p className="empty-state">No findings match this filter.</p>
                )}
              </div>
              <article className="remediation-panel" aria-labelledby="remediation-title">
                <div className="detail-header">
                  <div>
                    <p className="eyebrow">{selectedFinding.ruleId} / {selectedFinding.pack}</p>
                    <h3 id="remediation-title">{selectedFinding.title}</h3>
                  </div>
                  <span className={`outcome-chip outcome-${selectedFinding.outcome}`}>
                    <StatusMark status={selectedFinding.outcome} />
                    {outcomeLabels[selectedFinding.outcome]}
                  </span>
                </div>
                <p>{selectedFinding.summary}</p>
                <div className="standard-link"><ShieldIcon /><span>Control source<strong>{selectedFinding.standard}</strong></span></div>
                <h4>Recommended remediation</h4>
                <ol>
                  {selectedFinding.remediation.map((step) => <li key={step}>{step}</li>)}
                </ol>
                <div className="linked-evidence">
                  <span>Linked evidence</span>
                  <div>
                    {selectedFinding.evidenceIds.map((id) => {
                      const item = data.evidence.find((evidence) => evidence.id === id);
                      return item ? (
                        <button key={id} onClick={() => setSelectedEvidenceId(id)} type="button">
                          <EvidenceIcon /> {item.label}
                        </button>
                      ) : null;
                    })}
                  </div>
                </div>
              </article>
            </div>
          </section>

          <section className="section" id="evidence" aria-labelledby="evidence-title">
            <div className="section-heading">
              <div>
                <p className="eyebrow">REPRODUCIBLE ARTIFACTS</p>
                <h2 id="evidence-title">Evidence explorer</h2>
              </div>
              <span className="integrity-label"><LockIcon /> Content-addressed</span>
            </div>
            <div className="evidence-layout">
              <div className="evidence-list" role="list" aria-label="Evidence artifacts">
                {data.evidence.map((evidence) => (
                  <button className={evidence.id === selectedEvidence.id ? "is-selected" : ""} key={evidence.id} onClick={() => setSelectedEvidenceId(evidence.id)} role="listitem" type="button">
                    <span className="evidence-type"><EvidenceIcon /></span>
                    <span><strong>{evidence.label}</strong><small>{evidence.kind} · {evidence.capturedAt}</small></span>
                    {evidence.redacted && <LockIcon className="lock-small" />}
                  </button>
                ))}
              </div>
              <EvidenceDetail evidence={selectedEvidence} />
            </div>
          </section>

          <section className="section split-section" aria-label="Baseline and policy">
            <article className="baseline-panel" id="baseline">
              <div className="section-heading compact">
                <div><p className="eyebrow">REGRESSION SIGNAL</p><h2>Baseline comparison</h2></div>
                <span className="baseline-name">{data.baseline.name}</span>
              </div>
              <p>Compared with the approved report from {data.baseline.createdAt}.</p>
              <div className="delta-grid">
                {data.baseline.deltas.map((delta) => {
                  const change = delta.current - delta.baseline;
                  return (
                    <div className="delta" key={delta.label}>
                      <span>{delta.label}</span>
                      <strong>{delta.current}{delta.label === "Coverage" ? "%" : ""}</strong>
                      <small className={`tone-${delta.tone}`}>
                        {change === 0 ? "—" : change > 0 ? `↑ ${change}` : `↓ ${Math.abs(change)}`} vs {delta.baseline}{delta.label === "Coverage" ? "%" : ""}
                      </small>
                    </div>
                  );
                })}
              </div>
            </article>
            <article className="policy-panel" id="policy">
              <div className="section-heading compact">
                <div><p className="eyebrow">DECISION LOGIC</p><h2>Policy gate</h2></div>
                <ShieldIcon />
              </div>
              <div className="policy-title">
                <span><strong>{data.policy.name}</strong><small>Version {data.policy.version}</small></span>
                <span className="signed-label">✓ Signed</span>
              </div>
              <ul>
                <li><StatusMark status="blocker" /><span><strong>Blocker rule</strong>{data.policy.blockerRule}</span></li>
                <li><StatusMark status="warning" /><span><strong>Warning budget</strong>{data.policy.warningBudget}</span></li>
                <li><StatusMark status="inconclusive" /><span><strong>Evidence minimum</strong>{data.policy.evidenceMinimum}</span></li>
              </ul>
              <button className="text-button" type="button">Inspect policy manifest <ChevronIcon /></button>
            </article>
          </section>

          <section className="section audit-section" id="audit" aria-labelledby="audit-title">
            <div className="section-heading">
              <div><p className="eyebrow">APPEND-ONLY HISTORY</p><h2 id="audit-title">Audit activity</h2></div>
              <button className="text-button" type="button">View full audit log <ChevronIcon /></button>
            </div>
            <div className="audit-table" role="table" aria-label="Recent audit activity" tabIndex={0}>
              <div className="audit-row audit-head" role="row">
                <span role="columnheader">Time (UTC)</span><span role="columnheader">Actor</span><span role="columnheader">Action</span><span role="columnheader">Resource</span>
              </div>
              {data.audit.map((event) => (
                <div className="audit-row" role="row" key={event.id}>
                  <span role="cell">{event.timestamp}</span><strong role="cell">{event.actor}</strong><span role="cell">{event.action}</span><code role="cell">{event.resource}</code>
                </div>
              ))}
            </div>
          </section>

          <footer>
            <span><PulseIcon /> FHIR Flightcheck · synthetic demonstration</span>
            <span>Technical readiness, not compliance certification</span>
            <a href="#overview"><ArrowUpIcon /> Back to top</a>
          </footer>
        </div>
      </main>
    </div>
  );
}
