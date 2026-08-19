# Architecture

FHIR Flightcheck separates orchestration, evaluation, and verification so a
readiness result can be traced to pinned rules and observed evidence. This page
describes the current `0.1.0` implementation. Proposed controls are called out
explicitly and are not runtime claims.

## Current system

```text
                 public bearer token
Go CLI --------------------------------------+
                                              |
Static Next.js demo                           v
(not API-connected)                  Go control plane
                                      |      |      |
                                      |      |      +-- Ed25519 report signer
                                      |      |
                                      |      +-- rule-pack catalog
                                      |
                                      +-- PostgreSQL transaction
                                           | run + job + outbox
                                           v
                                    outbox dispatcher
                                           |
                                           v
                                     NATS JetStream
                                           |
                                    worker bearer token
                                           v
                                    Python evaluator
                                           |
                                  bounded HTTP, no redirects
                                           v
                                   authorized FHIR target
```

### Control plane

The Go service owns projects, targets, runs, jobs, findings, evidence metadata,
reports, baselines, and idempotency records. In the assembled Compose path it
uses PostgreSQL and a fixed `org_local` organization.

A mutating public request requires an 8-to-128-character idempotency key. Run
creation writes the run, evaluation job, and outbox record in one PostgreSQL
transaction. The dispatcher publishes the outbox payload to the
`FLIGHTCHECK_JOBS_V1` JetStream stream and marks the message published only
after broker acknowledgement.

Public API routes use the API bearer token in durable mode. Worker completion
uses a different token and `/internal/v1/jobs/{jobID}/complete`. Health and
readiness routes are unauthenticated.

### Evaluator

The Python worker uses a pull consumer, fetching up to eight JetStream messages.
It validates the job contract, runs the catalog's deterministic evaluators,
submits findings and evidence metadata to the control plane, then acknowledges
the message. Transient completion failures are negatively acknowledged with
bounded exponential delay; malformed, permanent, or exhausted work is
terminated.

Rules declare `network` or `fixtures` capabilities in the current packs. The
catalog rejects startup rules requesting target credentials or writes. Network
probing uses bounded HTTPX timeouts and does not follow redirects.

### Contracts

JSON Schema Draft 2020-12 under `packages/contracts/schema` is intended as the
cross-language source. Generated Go, Python, and TypeScript artifacts are
committed and drift-checked. Runtime domain models also validate untrusted
inputs; generated types alone are not a security boundary.

Every run manifest pins schema version `1.0.0`, organization, project, target,
profile, rule IDs/versions, and creation time. The current public API supports
only profile `startup-r4` and FHIR R4 `4.0.1`.

### Decisions and signatures

The control plane accepts at most one finding per selected rule and validates
the finding's pinned version, severity, title, remediation, run ID, and evidence
references. It sorts findings and evidence before completing the job.

Current fixed policy:

- failed `critical` or `high` finding: `not_ready`;
- other failure or warning: `conditional`;
- any `inconclusive` or `platform_error`: `incomplete`;
- otherwise: `ready`.

Incomplete coverage always forces `incomplete`. Only complete reports receive
an Ed25519 signature. The report includes a SHA-256 hash of the run manifest;
the CLI verifies contract shape and signature with a configured public key.

### Web console

The Next.js application renders a detailed blocker-first interface from static
synthetic data in `apps/web/src/lib/demo-data.ts`. It is useful for reviewing
the intended information architecture and interaction design, but it does not
read live projects, runs, evidence, policy, or audit data. Its action buttons
are not operational.

### Data stores

- PostgreSQL is the durable source for current control-plane state and the
  transactional outbox.
- NATS JetStream carries evaluator jobs with at-least-once delivery.
- Garage starts in Compose but is not used by the application path. Evidence
  metadata points to content-addressed `urn:sha256:` identifiers; artifact
  bodies are not uploaded.
- HAPI FHIR is a local synthetic network target, not a dependency of the
  control plane itself.

## Run lifecycle

1. The CLI creates a project and records the control-plane public signing key.
2. The CLI registers an HTTP(S) FHIR target. Private/local addresses require an
   explicit demo flag accepted only when the server enables local demo policy.
3. The CLI creates a `startup-r4` run with a fresh idempotency key.
4. The control plane snapshots the target and rule versions into the manifest,
   then commits run, job, and outbox state together.
5. The dispatcher publishes the job to JetStream.
6. The evaluator performs declared network/fixture checks and submits findings
   plus evidence metadata with the worker token.
7. The control plane validates completion, computes coverage and decision,
   signs a complete report, and commits terminal state.
8. The CLI polls for at most two minutes, downloads the report, and optionally
   saves JSON.
9. Offline verification checks schema/contract invariants and the Ed25519
   signature. A complete signed report can be selected as a baseline.
10. `flightcheck ci --against baseline` verifies both reports and fails on a
    worse decision, a worsened matching-rule outcome, incomplete coverage, or
    signature failure.

## Failure semantics

Target findings remain distinct from evaluator/control-plane failure. A missing
or ambiguous observation is `inconclusive`; a Flightcheck execution defect is
`platform_error`; neither can become `pass`. Missing terminal findings force an
unsigned `incomplete` report.

At-least-once delivery means evaluation may repeat. The outbox uses a stable
broker message ID, the worker suppresses repeats during one process lifetime,
and PostgreSQL completion logic is the durable authority. Default packs contain
no active writes, so redelivery cannot intentionally mutate a clinical target.

## Trust boundaries

The implemented boundaries are:

- separate public API and worker completion bearer tokens;
- strict JSON decoding, body limits, request deadlines, and idempotency keys;
- target URL validation and an explicit private-target exception;
- no redirects in evaluator HTTP;
- no target credentials or write capabilities in startup manifests;
- non-root control-plane, CLI, and evaluator containers;
- report integrity through SHA-256 manifest binding and Ed25519 signatures.

These are not a complete production security model. OIDC, RBAC, RLS, workload
identity, credential brokerage, envelope encryption, KMS/HSM signing, live
artifact storage, per-rule isolation, network egress enforcement, and
multi-tenant authorization remain proposed. See
[ADR 0005](0005-identity-isolation-and-key-management.md) and the
[limitations](../LIMITATIONS.md).

## Known architecture gaps

- The queued job currently contains an empty fixture, so bundled fixture
  scenarios are not selected through the public run API.
- Garage/S3 environment values are composed but not consumed.
- The web console and control plane are not integrated.
- Health readiness does not check the broker, workers, or artifact storage.
- The worker's local completion cache and terminated-message handling do not
  provide an operator-facing dead-letter workflow.
- One fixed organization and static service tokens prevent production
  multi-tenancy.

## Decision records

- [ADR 0001: S3-compatible artifact storage](0001-s3-compatible-artifact-store.md)
  — accepted design; application integration is pending.
- [ADR 0002: versioned JSON contracts](0002-versioned-contracts.md)
- [ADR 0003: at-least-once rule execution](0003-at-least-once-rule-execution.md)
- [ADR 0004: readiness without scorewashing](0004-readiness-without-scorewashing.md)
- [ADR 0005: identity, isolation, and key management](0005-identity-isolation-and-key-management.md)
  — proposed production design.
- [Service-level objectives](slos.md) — objectives/design targets until measured
  production operation exists.
