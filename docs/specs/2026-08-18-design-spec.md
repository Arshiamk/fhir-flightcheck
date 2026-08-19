# FHIR Flightcheck Design

**Status:** Approved for implementation planning  
**Owner:** Arshiamk  
**Date:** 2026-08-18  
**License:** Apache-2.0

## 1. Executive summary

FHIR Flightcheck is an open-source production-readiness platform for healthtech
teams. It evaluates a FHIR R4/SMART integration across interoperability,
reliability, security, auditability, and guardrailed AI behavior. It produces
reproducible evidence, actionable remediation, and regression gates for CI.

The product is not a compliance certificate, clinical safety approval, medical
device, or legal assessment. It measures explicitly defined technical controls
and preserves the evidence behind every result.

The repository is intended to demonstrate CTO-level healthtech engineering:
standards fluency, distributed systems, secure multi-tenancy, evidence-based AI
governance, operability, supply-chain security, and disciplined product scope.

## 2. Problem and target user

### Primary user

An engineering lead or platform team at a healthtech startup preparing to
integrate with an EHR, health system, payer, or clinical AI workflow.

### Problem

Teams currently assemble separate validators, hand-written integration tests,
security checklists, load scripts, and AI evaluations. Results are difficult to
reproduce, rarely share a common evidence model, and are usually disconnected
from release gates. Failures are therefore discovered during customer
onboarding, security review, or production incidents.

### Product promise

> Know whether a healthtech integration is technically ready before a customer,
> auditor, or production incident finds the gaps.

### Success criteria

1. A new user can launch the synthetic demo and obtain a useful report in under
   ten minutes.
2. Every finding can be reproduced from a pinned run manifest.
3. Every failure includes evidence, severity, rule provenance, and remediation.
4. A baseline can gate a pull request without requiring the web console.
5. No real patient data or paid model API is needed for the complete demo.
6. The architecture can scale from Docker Compose to a multi-tenant deployment
   without changing rule semantics.

## 3. Research basis and differentiation

FHIR adoption is moving from pilots toward governed implementation, while AI
validation and governance remain weaker than basic FHIR implementation. The
design is informed by:

- HL7 FHIR R4 and its implementer safety checklist.
- SMART App Launch authorization patterns.
- The 2026 State of FHIR findings on interoperability as a prerequisite for AI.
- FHIRTrustBench's separation of FHIR quality, AI validation, workflow
  integration, trustworthiness, and governance readiness.
- Existing FHIR validators, test cases, workflow engines, and healthcare AI
  SDKs.

Flightcheck does not compete by reimplementing a full FHIR validator. It
orchestrates trusted validators where appropriate and adds operational probes,
cross-control evidence, failure recovery tests, AI guardrail tests, signed
reports, historical baselines, and CI enforcement.

The durable differentiator is the combination of:

1. deterministic standards checks;
2. controlled active failure testing;
3. a common evidence and remediation model;
4. transparent, policy-driven readiness decisions;
5. regression tracking across versions; and
6. an extensible but capability-restricted rule SDK.

## 4. Scope

### Version 1

- FHIR R4 4.0.1 targets.
- SMART discovery and authorization configuration checks.
- Local, sandbox, and explicitly authorized remote targets.
- Synthetic Synthea-derived fixtures.
- Four first-party packs:
  - FHIR conformance and data quality;
  - reliability and recovery;
  - security, privacy engineering, and auditability;
  - AI safety and human oversight.
- At least 25 meaningful deterministic rules.
- Go CLI and control-plane API.
- Python evaluator workers and rule SDK.
- Next.js/TypeScript operations console.
- PostgreSQL, NATS JetStream, and S3-compatible evidence storage.
- Docker Compose local environment.
- GitHub Action, JSON, SARIF, and human-readable reports.
- Baseline comparison and policy gates.
- OpenTelemetry traces, Prometheus metrics, and structured redacted logs.

### Explicit non-goals

- HIPAA, FDA, SOC 2, ISO, or legal certification.
- Clinical diagnosis, treatment recommendations, or patient-facing advice.
- Replacement of an EHR, FHIR server, SIEM, or general vulnerability scanner.
- Uncontrolled penetration testing.
- Storage of production FHIR payloads by default.
- Automated writes to clinical systems in the default rule packs.
- Kubernetes as a local-development requirement.
- FHIR R5 support in the first stable release.

## 5. User experience

### CLI path

```text
flightcheck init
flightcheck target add
flightcheck run --profile startup-r4
flightcheck report open
flightcheck baseline set
flightcheck ci --against baseline
```

`init` creates a reviewed configuration with safe defaults. `target add`
performs discovery without persisting credentials in plaintext. `run` prints
live progress and a final decision summary. `ci` returns stable documented exit
codes.

### Console path

The console provides:

- guided target setup and scope preview;
- run progress grouped by evaluation pack;
- blocker-first results rather than a vanity score;
- an evidence explorer with redaction indicators;
- remediation steps and links to relevant standards;
- baseline and regression views;
- report export and verification; and
- administrative policy, identity, and audit views.

The visual system must meet WCAG 2.2 AA. Color is never the only status signal,
text contrast is tested automatically, and all run/report workflows are
keyboard accessible.

### Demo path

The bundled demo includes one healthy target and three intentionally broken
scenarios:

1. invalid references and terminology;
2. pagination, throttling, and transient failure mishandling;
3. overbroad authorization plus an AI workflow attempting an unsafe write.

Users can apply documented remediations and see the report improve.

## 6. Architecture

### Repository layout

```text
apps/
  web/                  Next.js operations console
cmd/
  flightcheck/          Go CLI
services/
  control-plane/        Go API and orchestration
workers/
  evaluator/            Python worker and rule SDK
packages/
  contracts/            Versioned JSON Schemas and generated types
  rule-packs/           First-party rule definitions
fixtures/
  synthea/              Reviewed synthetic clinical data
deploy/
  compose/              One-command local environment
  helm/                 Production deployment after the local path is stable
docs/
  architecture/         ADRs, diagrams, threat model, SLOs
  rules/                Rule authoring and evidence model
```

### Components

#### Go control plane

Owns organizations, projects, targets, encrypted credentials, policies, rule
catalogs, run manifests, job state, evidence indexes, readiness decisions,
baselines, audit events, and public APIs.

It is the sole authority for state transitions. Every mutating request accepts
an idempotency key. State changes and job publication use a transactional
outbox so database commits cannot diverge from dispatched work.

#### Go CLI

Provides local and CI workflows using the same public API contracts as the web
console. It supports machine-readable output, non-interactive authentication,
stable exit codes, report verification, and an embedded local-demo launcher.

#### Python evaluator workers

Execute versioned checks in isolated processes. Workers claim at-least-once
jobs, verify capability grants, enforce resource budgets, redact evidence, and
submit results through the control plane. A deterministic check never requires
an LLM.

#### TypeScript web console

Consumes generated API types. Server-side code handles session boundaries;
browser code never receives target credentials. Progress uses server-sent
events with reconnect support and polling fallback.

#### Data services

- PostgreSQL is the source of truth for transactional state.
- NATS JetStream carries at-least-once work and progress events.
- S3 or MinIO stores content-addressed evidence and report artifacts.
- OpenTelemetry connects API, job, rule, and outbound-request traces.

## 7. Contracts and data model

### Core entities

- `Organization`: tenant and policy boundary.
- `Project`: groups targets, policies, and baselines.
- `Target`: endpoint metadata and encrypted credential reference.
- `RulePack`: signed collection of versioned rules.
- `Rule`: declaration, capability requirements, and evaluator reference.
- `EvaluationPlan`: resolved rules and policy for a run.
- `RunManifest`: immutable versions, target snapshot, and fixture/model inputs.
- `RuleExecution`: attempt state, timing, and failure classification.
- `Finding`: outcome, severity, evidence references, and remediation.
- `Evidence`: redacted content-addressed artifact and provenance.
- `Report`: signed summary and verification metadata.
- `Baseline`: approved report reference for regression comparison.
- `AuditEvent`: append-only security and administrative event.

All tenant-owned tables include an organization identifier. Authorization is
enforced in the application and backed by database row-level policies as
defense in depth.

### Rule contract

Each rule declares:

- globally stable rule identifier and semantic version;
- supported FHIR versions and implementation-guide packages;
- required network, credential, fixture, model, and write capabilities;
- passive, active-read, or active-write behavior;
- timeout and bounded resource budget;
- deterministic or probabilistic execution;
- input and evidence schemas;
- severity and decision semantics;
- remediation metadata; and
- deprecation/replacement information.

The boundary uses versioned JSON Schema. Go, Python, and TypeScript types are
generated and checked for compatibility in CI.

### Finding outcomes

- `pass`: required evidence proves the condition.
- `fail`: evidence proves the condition is violated.
- `warning`: a non-blocking risk is demonstrated.
- `not_applicable`: the rule does not apply, with a recorded reason.
- `inconclusive`: required evidence was unavailable or ambiguous.
- `platform_error`: Flightcheck could not execute the rule correctly.

Unavailable evidence can never silently become `pass`.

### Readiness decisions

Flightcheck reports per-pack coverage and outcomes. A policy computes
`ready`, `conditional`, or `not_ready` from explicit blocker rules and tolerated
warnings. A single aggregate percentage is not used as a compliance proxy.

## 8. Run lifecycle

1. Authenticate and authorize the caller.
2. Resolve the target, profile, rule-pack versions, policy, and fixtures.
3. Validate requested capabilities and require confirmation for active probes.
4. Create an immutable run manifest and transactional outbox records.
5. Dispatch idempotent rule executions.
6. Workers obtain short-lived scoped access and execute within budgets.
7. Evidence is redacted, hashed, uploaded, and schema-validated.
8. The control plane records findings and streams progress.
9. Exhausted transient failures move to a dead-letter workflow.
10. When all rules are terminal, compute policy decisions and coverage.
11. Create and sign the report manifest.
12. Compare with the selected baseline and emit CI/SARIF results.

Cancellation stops undispatched work and asks active workers to terminate. A
cancelled run remains inspectable. Resuming creates new attempts under the same
manifest only when inputs are unchanged; otherwise it creates a new run.

## 9. Evaluation packs

### FHIR conformance and data quality

- capability statement discovery and consistency;
- content type and FHIR version behavior;
- profile validation through trusted validator integration;
- reference integrity and contained-resource behavior;
- terminology binding and code-system correctness;
- search parameter, pagination, and bundle-link behavior;
- date, timezone, precision, and narrative-safety checks;
- stable handling of unknown extensions and modifier extensions; and
- representative US Core profile checks through pinned packages.

### Reliability and recovery

- idempotent retry behavior;
- throttling and `Retry-After` handling;
- timeout, connection reset, and partial-response behavior;
- pagination loops and duplicate-page detection;
- optimistic concurrency/version conflict behavior;
- webhook/subscription replay and signature handling where supported;
- bounded concurrency and backpressure; and
- recovery-point evidence without destructive writes.

### Security, privacy engineering, and auditability

- SMART metadata and redirect configuration;
- requested scope minimization;
- token audience, issuer, expiry, and refresh handling;
- tenant and patient-context boundary tests in synthetic environments;
- TLS and security-header observations;
- secret, token, and PHI redaction tests;
- audit-event completeness and correlation;
- retention-policy and data-export evidence declarations; and
- SSRF and unsafe redirect defenses in Flightcheck itself.

These checks assess technical behavior. They do not certify regulatory
compliance.

### AI safety and human oversight

- response grounding against supplied FHIR facts;
- unsupported clinical claim detection;
- citation/resource provenance;
- prompt-injection resistance from synthetic resource text;
- prohibited autonomous write attempts;
- least-privilege tool use;
- human-review routing for configured risk classes;
- model/version traceability;
- deterministic output-schema checks; and
- cross-model evaluation only as supporting evidence.

Golden cases and rule-based assertions are primary. LLM judges are optional,
calibrated, and never the sole basis of a blocking decision.

## 10. Security and privacy model

### Safe defaults

- Synthetic data is the default and complete demo path.
- Target payload persistence is disabled unless explicitly enabled.
- Secrets are stored through envelope encryption and never logged.
- Workers receive short-lived, least-privilege credentials.
- Active-write rules are disabled in first-party default profiles.
- Reports carry sensitivity labels and redaction summaries.

### Platform protections

- OIDC for users and workload identity for services.
- RBAC plus project-scoped policy checks.
- Per-organization data boundaries with database defense in depth.
- Endpoint validation, DNS/IP revalidation, private-range controls, redirect
  limits, and configurable egress allowlists to mitigate SSRF.
- Signed webhooks, replay protection, rate limits, and idempotency keys.
- Content-addressed artifacts and signed report manifests.
- Append-only audit records exported to an operator-controlled sink.
- Dependency pinning, SBOMs, image signing, and build provenance.

### Threat model

The repository must include a STRIDE-based threat model covering malicious
targets, compromised rule packs, credential theft, cross-tenant access,
evidence tampering, prompt injection, queue replay, supply-chain compromise,
and denial of service. High-risk mitigations must have executable tests.

## 11. Error handling

Errors are classified as:

- user/configuration errors;
- target findings;
- transient target failures;
- permanent target failures;
- rule defects;
- platform/infrastructure failures; and
- authorization or policy denials.

Only transient failures are retried, using bounded exponential backoff with
jitter and a total deadline. Rule execution is idempotent by run, rule, and
attempt identity. Duplicate completion messages are harmless. Platform errors
remain visibly separate from target failures and never reduce the target's
readiness rating.

Partial reports show completed coverage and missing evidence. They cannot be
signed as final readiness reports.

## 12. Observability and operations

- Trace context propagates from incoming request through job dispatch, worker,
  outbound FHIR request, evidence upload, and report creation.
- Logs are structured, correlated, and redacted at source.
- Metrics cover queue lag, run latency, rule duration, retry rate, dead letters,
  report completion, artifact failures, and redaction events.
- Initial documented SLOs cover API availability, accepted-to-start latency,
  report completion, and evidence durability.
- Health endpoints distinguish liveness, readiness, and dependency degradation.
- Runbooks cover queue backlog, worker quarantine, signing-key rotation,
  artifact recovery, and suspected cross-tenant access.

## 13. Testing strategy

### Per-language checks

- Go: formatting, static analysis, race detector, unit and integration tests.
- Python: formatting, linting, type checking, unit/property tests, package
  integrity.
- TypeScript: formatting, linting, strict type checking, unit/component tests.

### Cross-system checks

- JSON Schema compatibility and generated-type drift tests.
- Contract tests across Go, Python, and TypeScript.
- HAPI FHIR integration tests with pinned containers.
- Golden Synthea fixtures and malformed-bundle corpora.
- Fuzz/property tests for bundles, pagination, references, and evidence parsing.
- Fault injection for timeouts, resets, duplicates, queue replay, and storage
  failure.
- End-to-end CLI and browser flows.
- WCAG 2.2 AA automated checks plus keyboard-flow tests.
- Authorization and cross-tenant negative tests.
- Report signature and backward-compatibility tests.
- Load tests with explicit API, queue, and report-generation budgets.

The CI matrix runs on Linux, Windows, and macOS where relevant. Main is always
releasable.

## 14. Delivery and release quality

Required pull-request checks include formatting, linting, type checking, tests,
secret scanning, dependency review, license policy, SAST, container scanning,
SBOM generation, and contract compatibility.

Releases use semantic versioning and include:

- signed CLI binaries for Linux, macOS, and Windows;
- signed multi-architecture containers;
- checksums, SBOMs, and provenance attestations;
- migration and rollback notes;
- a rule-pack and FHIR-package compatibility matrix; and
- generated release notes with human-reviewed highlights.

Rule identifiers remain stable. Rule behavior changes require a semantic
version change and migration notes. Reports pin platform, rule, profile,
fixture, terminology/package, and optional model versions.

## 15. Documentation and adoption

The repository ships with:

- a concise, outcome-led README;
- a reproducible terminal demo and screenshots;
- a complete example report and verification command;
- architecture diagrams and ADRs;
- threat model and privacy model;
- rule-authoring and capability-security guides;
- local and production deployment guides;
- troubleshooting and runbooks;
- explicit limitations and non-certification language;
- public roadmap and release cadence; and
- benchmark methodology with reproducible synthetic inputs.

Adoption will be earned through utility rather than star-gaming. The launch
artifact is the CLI and synthetic demo. Follow-up releases can publish
reproducible findings across public FHIR test servers, add community-requested
rules, and provide shareable readiness badges that link to verifiable reports.

## 16. Repository governance and authorship

Version 1 is a single-author project owned by Arshiamk. Issues and design
feedback may be accepted, but external code commits are not merged while
single-author history is required. Third-party dependencies retain their own
copyright and license notices; they are not repository commit authors.

All commits created for this project use Arshiamk's GitHub-verified commit
identity and contain no AI attribution or `Co-authored-by` trailers. The
verified email must be confirmed before the first commit so GitHub attribution
is reliable.

## 17. Version 1 acceptance criteria

Version 1 is complete only when:

1. `docker compose up` launches the complete synthetic demo.
2. The CLI produces a useful report in under ten minutes on supported hardware.
3. Four packs provide at least 25 reviewed deterministic rules.
4. Three broken scenarios fail for the intended reasons and pass after the
   documented remediations.
5. Reports are reproducible, signed, exportable, and verifiable offline.
6. Baseline regression gating works in GitHub Actions.
7. No default workflow requires real PHI, destructive probes, or a paid model.
8. Security, tenant-isolation, contract, fault-injection, accessibility, and
   end-to-end suites pass.
9. All release artifacts include signatures, SBOMs, and provenance.
10. The README, architecture, threat model, limitations, rule guide, deployment
    guide, and example report are complete and tested.

## 18. Implementation sequencing

Implementation planning should split work into reviewable vertical slices:

1. contracts, monorepo foundations, and synthetic target;
2. local control plane, CLI, and one end-to-end deterministic rule;
3. evidence storage, report model, signing, and verification;
4. worker isolation, JetStream delivery, retries, and observability;
5. four rule packs and broken-target scenarios;
6. console, baseline comparison, SARIF, and GitHub Action;
7. threat-model mitigations, fault injection, accessibility, and performance;
8. documentation, packaging, release security, and launch polish.

Each slice must produce a demonstrable user outcome and preserve a green,
releasable main branch.
