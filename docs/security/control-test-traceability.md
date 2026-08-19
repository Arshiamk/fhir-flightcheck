# Security control-to-test traceability

This is the required verification contract for the target architecture. A row
marked `planned` is an acceptance test to implement, not evidence that the
control exists. CI job names are evidence only when they ran successfully on
the exact commit; a skipped job is not a pass.

## Verification snapshot

Reconciled against the repository on 2026-08-19. Local runs passed:

- Go: `go test ./cmd/flightcheck/... ./services/control-plane/...`;
- Python: 38 tests in `workers/evaluator/tests`;
- web unit/component: 6 tests; and
- Playwright accessibility/keyboard flows: 6 tests across Chromium desktop and
  mobile projects.

The PostgreSQL test
`services/control-plane/postgres_test.go::TestPostgresRepositoryIdempotencyAndTenantIsolation`
was explicitly skipped because `FLIGHTCHECK_TEST_DATABASE_URL` was not set, and
the standard CI workflow does not provision PostgreSQL. Browser E2E tests exist
but are not invoked by `.github/workflows/ci.yml`. GitHub-hosted security and
release workflow results were not available to this local verification, so
their rows say **configured**, not passing.

| Test ID | Threat/control | Required executable evidence | Stage | Status |
|---|---|---|---|---|
| CT-SSRF-01 | TM-01 URL parser | Unit/property tests reject non-HTTP schemes, userinfo, encoded/alternate IP forms, loopback, link-local, multicast, unspecified, private and configured internal ranges for IPv4/IPv6. | PR | Partial, passing: `services/control-plane/security_test.go` covers literal loopback/private IPv4, IPv6 loopback, dual local opt-in, userinfo, query and fragment; `workers/evaluator/tests/test_http.py` covers an unapproved localhost target. Alternate encodings and the full address corpus remain planned. |
| CT-SSRF-02 | TM-01 DNS policy | Integration test uses controlled DNS to rebind between validation and connect; connection is pinned or denied. Mixed safe/unsafe answers fail closed. | PR/nightly | Planned |
| CT-SSRF-03 | TM-01 discovered URLs | Corpus test applies identical policy to redirects, pagination, canonical, attachment, terminology, webhook and model-generated URLs. | PR | Planned |
| CT-SSRF-04 | TM-01/02 redirects | Integration test caps hops, rejects downgrade/cross-origin by policy, strips authorization on origin change, and revalidates DNS/TLS each hop. | PR | Planned |
| CT-SSRF-05 | TM-01 network boundary | Deployment test proves the worker cannot reach metadata, control-plane, database, queue, or denied RFC1918 test endpoints even if application validation is bypassed. | Pre-release | Planned |
| CT-TARGET-01 | TM-02 target identity | TLS hostname/chain failures and issuer/audience mismatch fail; manifest records normalized target origin. | PR | Planned |
| CT-RULE-01 | TM-03 capability admission | Rules requesting undeclared network, credential, model, filesystem, subprocess or write capabilities are denied before dispatch. | PR | Partial, passing: `services/control-plane/rules_test.go::TestRuleCatalogRejectsUnsafeCapabilities` rejects an active-write rule and `workers/evaluator/tests/test_evaluator.py::test_registry_rejects_ungranted_capability` rejects an ungranted evaluator capability. The complete capability set remains planned. |
| CT-RULE-02 | TM-03 signature | Tampered, unsigned, unknown-publisher and revoked rule packs are rejected; exact accepted digest appears in manifest. | PR | Planned |
| CT-RULE-03 | TM-03 sandbox | Escape probes cannot write root, mount host paths, access container socket, add privileges, or outlive timeout. | Pre-release | Planned |
| CT-RULE-04 | TM-03 egress | Rule can reach only granted target origin and cannot use DNS/redirect as a second destination. | Nightly | Planned |
| CT-RULE-05 | TM-03 evidence gate | Oversized, invalid-schema, wrong-tenant, unredacted and undeclared evidence is rejected and audited. | PR | Planned |
| CT-RULE-06 | TM-03 writes | Default profile cannot schedule writes; write pool requires admin policy, confirmation, scoped credential and idempotency key. | PR | Partial, passing: `services/control-plane/rules_test.go::TestRuleCatalogRejectsUnsafeCapabilities` keeps active-write rules out of the current catalog. No write pool, approval, scoped write credential or target idempotency test exists. |
| CT-SECRET-01 | TM-04 credential boundary | Browser/API snapshot tests contain references only; manifests, queue envelopes and errors contain no plaintext credential. | PR | Partial, passing: `services/control-plane/http_test.go::TestAPIAuthenticationBoundary` proves API and worker tokens cannot cross endpoints; `services/control-plane/dispatch_test.go::TestJobEncodingMatchesWorkerContract` proves queued manifests use `credentialRef: none`; `workers/evaluator/tests/test_completion.py` proves worker-token errors are redacted. OIDC/browser and credential-broker boundaries remain planned. |
| CT-SECRET-02 | TM-04 encryption | Ciphertext changes with fresh nonce; wrong tenant/object associated data fails authentication; KEK/DEK purpose separation enforced. | PR | Planned |
| CT-SECRET-03 | TM-04 redaction | Synthetic canaries across structured/free-text/encoded inputs are absent from logs, traces, queue, evidence and reports. | PR/nightly | Partial, passing: `workers/evaluator/tests/test_evidence.py::test_redaction_precedes_hash_and_storage`, `workers/evaluator/tests/test_completion.py`, and `services/control-plane/security_test.go::TestRedact` cover structured evidence and error strings. End-to-end logs, traces, queue, reports, free text and encoded canaries remain planned. |
| CT-SECRET-04 | TM-04 grant expiry | Expired, wrong-destination, wrong-scope, reused and revoked worker grants fail. | PR | Planned |
| CT-SECRET-05 | TM-04 repository secrets | Gitleaks workflow scans repository changes/history as configured; GitHub secret scanning/push protection is separately enabled where available. | PR/platform | Configured, external run not verified: `.github/workflows/security.yml` uses SHA-pinned `gitleaks/gitleaks-action`; platform secret scanning and push protection are not asserted. |
| CT-TENANT-01 | TM-05 API isolation | For every tenant endpoint, read/write/export using another tenant's opaque IDs returns a non-disclosing denial and emits audit. | PR | Planned |
| CT-TENANT-02 | TM-05 RLS | Direct SQL under runtime role cannot select/insert/update/delete another tenant; missing context returns zero/denied, never global rows. | PR | Planned: forced RLS is defined in `services/control-plane/migrations/001_initial.sql`, but no direct-SQL runtime-role negative test exists. The conditional repository test in `services/control-plane/postgres_test.go` was skipped locally and in standard CI. |
| CT-TENANT-03 | TM-05 pool safety | Alternating tenant requests on one pooled connection never retain prior transaction context. | PR | Planned: transaction-local context is implemented in `services/control-plane/postgres.go`, but there is no alternating pooled-connection test. |
| CT-TENANT-04 | TM-05 relational integrity | Cross-tenant foreign-key references are rejected. | PR | Planned: tenant-qualified foreign keys exist in `services/control-plane/migrations/001_initial.sql`, but no executable cross-tenant foreign-key rejection test exists. |
| CT-TENANT-05 | TM-05 worker/object scope | Forged queue tenant, run, or object prefix cannot obtain grant, submit result, or read/upload object. | PR | Planned |
| CT-TENANT-06 | TM-05 privileged roles | Migration/runtime role inspection fails CI if runtime owns tenant tables or has superuser/`BYPASSRLS`. | PR | Planned: no role-inspection test or CI gate exists. |
| CT-EVIDENCE-01 | TM-06 content integrity | One-byte object mutation, wrong digest/key and swapped tenant/run reference fail verification. | PR | Partial, passing: `workers/evaluator/tests/test_evidence.py::test_canonical_hash_is_deterministic` proves canonical content hashing. There is no stored-object read-time digest, key/reference swap or one-byte mutation test. |
| CT-EVIDENCE-02 | TM-06 report integrity | Offline verifier rejects altered manifest, finding, coverage, evidence digest, signature, key ID or algorithm. | PR | Partial, passing: `services/control-plane/signing_test.go::TestReportSigningRoundTripAndTamperDetection` and `cmd/flightcheck/main_test.go::TestReportVerifyRejectsTampering` reject a changed signed decision. The complete field/key/algorithm tamper matrix remains planned. |
| CT-EVIDENCE-03 | TM-06 finality | Missing/nonterminal rules can produce only a labelled partial, unsigned readiness report. | PR | Implemented and passing: `services/control-plane/dispatch_test.go::TestIncompleteCoverageIsPersistedUnsigned`. |
| CT-EVIDENCE-04 | TM-06 immutability | Object version/lock policy test prevents overwrite and records deletion attempt. | Pre-release | Planned |
| CT-EVIDENCE-05 | TM-06 signing keys | Rotation fixtures verify old reports with trust policy and new reports only with active key; revoked-key behavior is explicit. | PR | Planned |
| CT-EVIDENCE-06 | TM-06 audit | Sensitive mutations create append-only export records with actor, scope, action, result and correlation ID but no payload/secrets. | PR | Planned |
| CT-QUEUE-01 | TM-07 deduplication | Duplicate delivery yields one authoritative completion and no duplicate finding/evidence index. | PR | Partial, passing: `workers/evaluator/tests/test_worker.py::test_duplicate_completion_is_acknowledged_without_reexecution` and duplicate/conflicting completion cases in `services/control-plane/http_test.go::TestAPIEndToEndAndIdempotency`. The worker idempotency store is process-local and no restart/JetStream/PostgreSQL integration test proves durable deduplication. |
| CT-QUEUE-02 | TM-07 envelope | Mutated tenant/run/rule/attempt/manifest digest or expired envelope is rejected. | PR | Planned |
| CT-QUEUE-03 | TM-07 stale attempts | Completion after cancellation, supersession or lease expiry cannot change final state. | PR | Partial, passing: wrong-run and conflicting duplicate completions are rejected in `services/control-plane/http_test.go::TestAPIEndToEndAndIdempotency`. Cancellation, supersession and lease-expiry paths are not tested. |
| CT-QUEUE-04 | TM-07 retry | Only classified transient errors retry with bounded exponential backoff/jitter/deadline, then DLQ. | PR | Partial, passing: `workers/evaluator/tests/test_completion.py` classifies transient HTTP/network failures; `workers/evaluator/tests/test_worker.py` verifies bounded backoff and terminal exhaustion; `services/control-plane/dispatch_test.go::TestOutboxDispatchIsRetryableAndIdempotent` verifies outbox retry retention. Jitter, total deadline and a real DLQ remain planned. |
| CT-QUEUE-05 | TM-07 side effects | Replayed active probe carries stable target idempotency key or is denied when safe replay is impossible. | Nightly | Planned |
| CT-QUEUE-06 | TM-07 permissions | Worker identity cannot publish arbitrary work or consume another worker class's subject. | Pre-release | Planned |
| CT-AI-01 | TM-08 injection | Adversarial synthetic resource text cannot change system policy, reveal canaries, or enable unauthorized tools. | PR | Planned: `workers/evaluator/tests/test_evaluator.py::test_broken_scenarios_fail_for_intended_controls` passes a fixture-declared prompt-injection assertion, but no model, secret canary or tool boundary is exercised. |
| CT-AI-02 | TM-08 authority | Model output alone cannot create a deterministic pass/blocker or invoke a clinical write. | PR | Planned: deterministic fixture rules for write prohibition pass in `workers/evaluator/tests/test_evaluator.py`, and active-write catalog admission is rejected by `services/control-plane/rules_test.go`, but no model-output authority boundary exists. |
| CT-AI-03 | TM-08 tool policy | Tool name and every argument are independently allowlisted and tenant/run/destination scoped. | PR | Planned |
| CT-AI-04 | TM-08 provenance | Report records prompt template, model/provider version, parameters, input digest and evaluator version without retaining restricted input by default. | PR | Planned: fixture-level model traceability is evaluated, but the report does not carry the required prompt/model/input provenance fields. |
| CT-AI-05 | TM-08 calibration | Golden/injection suite measures false pass/fail; configured high-risk outcomes route to human review. | Pre-release | Planned: fixture-level human-review routing is evaluated, but no model calibration or measured false-pass/false-fail suite exists. |
| CT-DOS-01..07 | TM-09 budgets | Tests cover slow headers/body, oversized/decompression payload, endless pages/cycles, retry storm, CPU/memory/disk/process limit, cancellation, and fair per-tenant queueing. | PR/nightly/load | Partial, passing: `services/control-plane/checker_test.go::TestCapabilityCheckerHonorsTimeout` and worker retry tests cover only timeout and retry bounds. Payload, decompression, pagination, process budgets, cancellation and tenant fairness remain planned. |
| CT-A11Y-01 | Accessible console | Axe WCAG 2.2 AA-tagged checks and keyboard navigation pass on supported desktop/mobile browser projects. | PR | Passing locally: `apps/web/e2e/operations-console.spec.ts` passed in Chromium desktop and mobile projects. Still not a required CI check because `.github/workflows/ci.yml` does not run `pnpm test:e2e`. |
| CT-SUPPLY-01 | TM-10 SAST | CodeQL analyzes Go, Python and JavaScript/TypeScript on PR/push/schedule. | CI | Configured, external run not verified: `.github/workflows/security.yml::codeql` uses SHA-pinned CodeQL actions for all three language groups. |
| CT-SUPPLY-02 | TM-10 dependency change | Dependency Review blocks high-severity vulnerable additions and denied licenses on pull requests where GitHub supports the feature. | PR | Configured, external run not verified: `.github/workflows/security.yml::dependency-review` is PR-only and SHA-pinned. |
| CT-SUPPLY-03 | TM-10 SBOM | Syft creates SPDX JSON for the source tree and uploads it as a short-retention CI artifact. | Push/PR | Configured, external run not verified: `.github/workflows/security.yml::sbom-and-filesystem` generates SPDX JSON and uploads it for 14 days using SHA-pinned actions. |
| CT-SUPPLY-04 | TM-10 vulnerability | Trivy scans filesystem/lockfiles; container-image scan becomes mandatory when buildable first-party Dockerfiles exist. | CI | Partial configuration, external run not verified: SHA-pinned Trivy filesystem scanning runs normally, but container scanning is manual and optional even though first-party Dockerfiles now exist. Mandatory first-party image scanning remains planned. |
| CT-SUPPLY-05 | TM-10 action authority | Workflow permission review confirms default `contents: read`; SARIF upload is isolated to jobs needing `security-events: write`; no pull-request secrets. | Review | Implemented by inspection: every `uses:` reference in `.github/workflows/ci.yml`, `security.yml`, and `release.yml` is pinned to a full commit SHA; default permissions are read-only and elevated permissions are job-scoped. No automated pin-policy test exists. |
| CT-SUPPLY-06 | TM-10 provenance | Release rehearsal verifies OIDC signing, provenance/SBOM subject digest, and signature without repository signing secrets. | Pre-release | Planned: `.github/workflows/release.yml` configures SHA-pinned OIDC provenance, SBOM and Cosign steps, but no successful release rehearsal was verified. |
| CT-AUDIT-01..03 | TM-11 accountability | Tests cover sensitive action completeness, append/export failure alerting, and log schema's secret/payload exclusion. | PR/nightly | Planned |

## Release rule

Before a feature moves from `planned` to production-enabled, its high-risk rows
must name a stable test location and pass in a required check. Exceptions need
an owner, compensating control, expiry, and issue link. Manual evidence can
supplement but not replace executable negative tests for tenant, SSRF,
credential, queue, evidence-integrity, sandbox, and model-authority boundaries.
