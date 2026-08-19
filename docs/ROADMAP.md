# Roadmap

FHIR Flightcheck is currently a `0.1.0` synthetic prototype. This roadmap is
ordered by risk reduction and usable outcomes, not dates. Items are planned
until they ship with tests and documentation; inclusion here is not a promise
or a claim that a control exists.

## Now: make the local path truthful and complete

Outcome: one documented Compose workflow evaluates an explicitly selected
bundled scenario and produces a report whose UI, CLI, API, and evidence agree.

- Pass a versioned synthetic fixture/scenario through the public run contract
  and immutable manifest instead of queueing an empty fixture.
- Make the healthy and three broken fixture scenarios runnable through the Go
  CLI and control plane.
- Connect the web console to the control-plane read APIs, clearly retain a
  separate static demo mode, and remove or disable non-functional actions.
- Persist redacted evidence bodies to the configured S3-compatible store,
  verify hashes on write/read, and test deletion and recovery.
- Populate rule-level standards references and distinguish normative
  requirements from Flightcheck policy.
- Add end-to-end tests for init, target creation, run completion, report export,
  offline verification, baseline selection, and CI exit codes.
- Capture current synthetic screenshots only after the live flow matches them.
- Run and publish the benchmark protocol with raw synthetic samples, environment
  metadata, and no extrapolated production claims.

Exit criteria:

- a clean checkout reaches a verified report by following the README;
- all 35 selected rules receive their declared inputs in each intended
  scenario;
- missing fixture, evidence, worker, and object-store dependencies fail visibly
  as `inconclusive` or `platform_error`, never `pass`;
- documentation examples are executed in CI or a release smoke test.

## Next: secure single-organization evaluation

Outcome: an isolated engineering team can assess an authorized non-production
target without relying on local-only security assumptions.

- Add OIDC user authentication, server-side sessions, project authorization,
  and auditable sensitive actions.
- Replace long-lived worker bearer tokens with workload identity or short-lived,
  scoped service credentials; support overlap rotation.
- Implement encrypted credential storage and destination/scope-bound delivery
  to workers without placing plaintext credentials in manifests or queues.
- Enforce HTTPS policy, DNS/IP revalidation, redirect/discovered-URL checks, and
  a documented egress proxy/firewall deployment.
- Harden evaluator containers with read-only filesystems, dropped privileges,
  resource budgets, and separate passive/read worker pools.
- Add a real dead-letter workflow, cancellation, configurable deadlines,
  operator-visible retries, and worker quarantine.
- Wire OpenTelemetry traces, bounded metrics, redacted structured logs, alerts,
  and tested local incident runbooks.
- Add official HL7 validator orchestration and pin implementation-guide and
  terminology packages by digest.

Exit criteria:

- threat-model controls for single-organization read-only evaluation have
  executable tests;
- a credential cannot be read by the browser, queue, unassigned worker, or
  report;
- a synthetic canary covers API, queue, evaluator, evidence storage, signing,
  and verification;
- load, soak, backup/restore, and key-rotation exercises have retained evidence.

## Later: production-grade multi-tenancy and key custody

Outcome: multiple organizations can share a deployment with independently
testable identity, data, and cryptographic boundaries.

- Add organization membership, roles, project policy, and deny-by-default
  authorization derived from verified identity.
- Enforce tenant-qualified foreign keys and PostgreSQL row-level security using
  non-owner runtime roles and transaction-local context.
- Scope queue grants and object keys to organization/run boundaries and add
  cross-tenant negative tests at every service boundary.
- Replace exportable report keys with non-exportable KMS/HSM signing, public-key
  publication, rotation overlap, revocation policy, and immutable key-use audit.
- Add envelope encryption and lifecycle management for retained evidence and
  target credentials.
- Support high availability, rolling contract compatibility, quotas, admission
  control, tenant-fair scheduling, and tested regional recovery.
- Export append-only security/audit events to an operator-controlled sink.
- Publish signed images and binaries with checksums, SBOMs, provenance, upgrade
  notes, rollback constraints, and a compatibility matrix.

Exit criteria:

- isolation tests demonstrate that a caller, job, worker, database session, and
  object reference cannot cross organization boundaries;
- signing and encryption key rotation and compromise recovery are exercised;
- SLOs are based on measured workloads and paired with alerts and error-budget
  policy;
- an external security review has addressed release-blocking findings.

## Future evaluation depth

Outcome: expand what Flightcheck can demonstrate without weakening the meaning
of existing reports.

- Add profile-specific FHIR R4 and implementation-guide packs only with pinned
  packages, provenance, and compatibility tests.
- Evaluate complete SMART launch, token lifecycle, patient context, and backend
  services against explicitly authorized sandboxes.
- Add reliability scenarios for pagination, throttling, retries, replay, and
  partial failure using controlled fault injection.
- Add deterministic AI workflow fixtures for grounding, provenance, injection,
  abstention, human review, and tool authority. Keep model judges optional and
  non-authoritative.
- Consider FHIR R5 in a separate versioned profile; do not silently reinterpret
  R4 reports.
- Consider active-write rules only in disposable synthetic environments with a
  separate worker class, explicit approval, target allowlist, and rule-specific
  idempotency.

## Explicitly not planned as shortcuts

- a percentage marketed as a compliance or safety score;
- automatic certification or legal conclusions;
- uncontrolled penetration testing or production clinical writes;
- sending target content to a model by default;
- accepting community rule code without provenance, capability review, and
  isolation;
- publishing performance numbers without raw synthetic data and a reproducible
  method.

See [Limitations](LIMITATIONS.md) for the current boundary and
[architecture](architecture/README.md) for the implemented/design split.
