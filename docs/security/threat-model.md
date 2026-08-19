# STRIDE threat model

**Status:** Design target; controls marked `planned` are not claims about the
current implementation.  
**Scope:** Control plane, web console, CLI, evaluator workers, rule packs,
PostgreSQL, JetStream, S3-compatible evidence storage, CI, and outbound FHIR or
model calls.

## Security objectives and trust boundaries

Flightcheck must keep one organization from observing or affecting another,
prevent credentials and patient data from escaping their intended boundary,
execute only authorized probes, and make final evidence and reports
independently verifiable. Availability is important, but never justifies
bypassing authorization, redaction, or integrity checks.

Principal boundaries are: browser to web server; CLI/web to control plane;
control plane to database, queue, and object store; queue to worker; worker to
an untrusted target; rule package to worker runtime; and CI to release
registries. A configured FHIR endpoint, its DNS, HTTP responses, resource text,
attachments, and links are attacker-controlled input. Community rules and
their dependencies are untrusted code until verified and admitted.

## Threat register

| ID | STRIDE | Threat and abuse path | Impact | Required controls | Verification | Residual risk |
|---|---|---|---|---|---|---|
| TM-01 | S/T/I/E | A malicious target changes DNS answers, redirects, or supplies absolute pagination/attachment URLs to reach loopback, link-local, private, metadata, or control-plane endpoints (SSRF). | Credential theft, internal discovery, tenant compromise. | Parse only `http`/`https`; reject userinfo and ambiguous IP forms; resolve all A/AAAA records; deny loopback, link-local, multicast, unspecified, private, carrier-grade NAT, and configured internal CIDRs unless an explicit administrator-approved sandbox policy allows them; re-resolve and re-check before every connection; pin the connection to an approved address while preserving TLS SNI/hostname validation; apply the same policy to redirects and every discovered URL; cap redirects; use an egress proxy/firewall as a second boundary. Never forward target authorization across origin changes. | CT-SSRF-01 through 05 | DNS rebinding and proxy misconfiguration remain possible; production needs network egress enforcement, not URL checks alone. |
| TM-02 | S/T/E | A target impersonates another endpoint through weak TLS, a misleading capability statement, or redirect. | Results attributed to the wrong system; credential disclosure. | HTTPS by production policy; normal PKI hostname verification; target identity snapshot in immutable manifest; redirect origin policy; token issuer/audience binding; no “ignore TLS” in hosted service. | CT-TARGET-01, CT-SSRF-04 | A validly issued certificate does not establish business ownership; onboarding approval remains necessary. |
| TM-03 | T/I/E | A malicious or compromised rule requests undeclared network, credential, model, filesystem, or write capability, escapes its process, or exfiltrates through evidence. | Target writes, secret/PHI theft, host compromise. | Signed pack digest and trusted publisher policy; manifest pins exact digest/version; deny-by-default capability grant; separate worker pools for passive/read/write classes; no host mounts or container socket; read-only root, non-root UID, dropped capabilities, seccomp/AppArmor where available; per-run ephemeral workspace; destination-bound credential broker; output schema, size, and redaction gate. Default first-party profiles prohibit writes. | CT-RULE-01 through 06 | Language-process isolation is not a security sandbox by itself; production requires an OS/container or microVM boundary. |
| TM-04 | I/E | Target credentials, OIDC tokens, queue credentials, signing keys, or plaintext data appear in browser state, logs, traces, exceptions, queue payloads, artifacts, or CI output. | Unauthorized target access and report forgery. | Browser receives references, never target credentials; envelope encryption; workload identity and short-lived grants; destination/scope-bound credential vending; structured allowlisted logging; redaction before persistence; secret scanner in CI; key use audit; prohibit secrets in queue messages and run manifests. | CT-SECRET-01 through 05 | Pattern redaction cannot detect every identifier or encoded secret; prevention and least privilege are primary. |
| TM-05 | S/E/I | Caller changes an organization/project identifier or a worker submits a result for another tenant. A connection pool leaks tenant context. | Cross-tenant disclosure or mutation. | OIDC subject mapped server-side; RBAC plus resource policy; derive tenant from authorization, never request body; organization ID on every tenant row and object key; PostgreSQL RLS with fail-closed transaction-local context; worker grant binds organization, run, rule, attempt, and object prefix; deny cross-tenant foreign keys; privileged maintenance role isolated and audited. | CT-TENANT-01 through 06 | Database owner/bypass-RLS roles can defeat RLS; runtime roles must not have either privilege. |
| TM-06 | T/R | An actor overwrites an evidence object, swaps an object reference, edits a finding, forges a report, or denies an administrative change. | False readiness decision and loss of auditability. | Canonicalize and hash redacted evidence; object key includes digest and tenant prefix; immutable/versioned object storage with retention where configured; verify digest on upload and read; final report signs manifest and artifact digest set with managed asymmetric key; append-only audit export; signatures include algorithm, key ID, and signing time; offline verifier rejects missing coverage or unknown/revoked keys according to policy. | CT-EVIDENCE-01 through 06 | Storage administrators can delete data unless object lock and an independently controlled backup/audit sink are enabled. A hash alone does not prove who produced evidence. |
| TM-07 | S/T/R/D | An attacker replays or mutates JetStream work/completion messages, or publishes a stale attempt after cancellation. | Duplicate target traffic, stale findings, inconsistent state, cost/availability impact. | TLS and per-service queue credentials; non-guessable message ID; signed or authenticated message envelope binding run/rule/attempt/tenant/manifest digest and expiry; durable consumer with bounded delivery; database state transition is authoritative; unique `(run, rule, attempt)` and inbox deduplication; compare-and-set completion; stale/cancelled attempts rejected; DLQ after bounded retries. | CT-QUEUE-01 through 06 | At-least-once delivery still causes duplicate execution unless side effects use idempotency keys or writes are prohibited. |
| TM-08 | T/I/E | FHIR narrative, resource text, retrieved documents, or rule content contains prompt injection that asks an optional model to reveal secrets, invoke tools, alter policy, or mark a result passed. | Data disclosure or unsafe/incorrect AI finding. | Treat target content as quoted data; fixed system policy and typed input; no credentials or general network tools in model context; tool allowlist with arguments independently authorized; output schema validation; prohibit autonomous clinical writes; deterministic checks and golden assertions are primary; LLM output cannot solely produce a blocking pass/fail; record model/prompt/version and route configured high-risk cases to humans. | CT-AI-01 through 05 | Models remain probabilistic and injection cannot be “solved” by prompting; isolation and authority limits carry the control. |
| TM-09 | D | Target streams huge/slow payloads, endless pagination, decompression bombs, recursive references, high-cardinality errors, or retry storms. Rules consume CPU, memory, disk, model budget, or queue capacity. | Worker starvation and delayed reports across tenants. | Connect/header/body/total deadlines; streamed byte and decompression limits; page/reference/depth limits and cycle detection; per-rule CPU/memory/process/disk/network/model budgets; per-tenant quotas and weighted scheduling; bounded concurrency; circuit breakers; retry budget with jitter; metric label allowlists; cancellation and quarantine. | CT-DOS-01 through 07 | An authorized tenant can consume its purchased quota; capacity planning and admission control remain operational controls. |
| TM-10 | T/E | A compromised dependency, action, base image, builder, package registry, rule pack, or release credential inserts code or alters artifacts. | Platform, target, or developer compromise. | Lockfiles and digest-pinned release inputs; Dependabot and dependency review; CodeQL and secret checks; SBOM and vulnerability scanning; minimal CI permissions; no secrets on untrusted pull requests; protected environments for release; build provenance and signing with OIDC-backed ephemeral identity; verify rule-pack and release signatures; two-person admission for new publishers/high-risk rules. | CT-SUPPLY-01 through 06 | Scanners have coverage gaps and vulnerability feeds lag; review and reproducible provenance remain necessary. |
| TM-11 | R | A user denies approving an active probe, changing policy, exporting evidence, rotating keys, or setting a baseline. | Unsafe activity and weak incident reconstruction. | Append-only, time-synchronized audit events with actor, tenant, action, target, request/trace ID, result, and reason; record confirmation and policy versions; export to operator-controlled sink; exclude payloads/secrets. | CT-AUDIT-01 through 03 | Application audit cannot independently prove events if the entire control plane is compromised. |

## Abuse-case rules

- A target response is never trusted merely because it is valid FHIR.
- `AllowPrivateNetwork` is a high-risk deployment exception, not a bypass
  supplied by a target or ordinary project member. It must be restricted to a
  sandbox worker pool with an explicit CIDR allowlist.
- Redirects, pagination links, canonical URLs, references, attachments,
  terminology endpoints, webhook destinations, and model-supplied URLs all pass
  the same outbound policy.
- A report with missing terminal rule executions is partial and cannot carry a
  final readiness signature.
- Active-write rules require a distinct worker class, target allowlist,
  least-privilege credential, idempotency strategy, and per-run confirmation.

## Review triggers

Review this model before enabling production FHIR payload retention, community
rules, active writes, new outbound protocols, model tools, shared worker pools,
or a new identity/storage/queue provider; and after any cross-tenant, credential,
evidence-integrity, or sandbox incident.
