# Data handling, redaction, and retention

## Boundary and defaults

FHIR Flightcheck is not a designated compliance archive. Synthetic fixtures are
the default. Production target payload persistence is disabled by default and
must be enabled by an organization administrator for a documented purpose.
Configuration does not make storage lawful or compliant; operators remain
responsible for authorization, contracts, regional requirements, and deletion
obligations.

Classify data as:

- **Restricted:** target credentials, tokens, keys, raw FHIR content, direct or
  quasi-identifiers, and unredacted request/response bodies.
- **Sensitive:** redacted evidence, findings, target URLs, tenant metadata,
  audit records, and reports that can reveal security posture.
- **Internal:** operational metrics and logs after validation that they contain
  no restricted data.
- **Public:** reviewed documentation, synthetic fixtures, and intentionally
  published release artifacts.

Sensitivity follows derived data. Hashes of low-entropy identifiers are still
sensitive and are not anonymization.

## Collection and redaction

Collect the minimum fields needed to prove a rule. Prefer a boolean, count,
schema path, status code, or keyed one-way correlation token over a resource
body. Workers redact before queueing or uploading. The control plane rejects
evidence that lacks a declared schema, sensitivity label, redaction status, and
size bound.

Redaction combines schema-aware removal with secret-pattern detection:

- drop FHIR narrative and attachment content unless a rule explicitly needs a
  synthetic sample;
- remove or replace names, identifiers, addresses, telecom, dates tied to a
  person, patient references, free text, URLs with query credentials, HTTP
  authorization/cookies, OAuth fields, and private keys;
- allowlist captured headers; denylist alone is insufficient;
- replace values with stable per-run keyed tokens only when correlation is
  necessary; use a run-scoped key so tokens cannot be joined across runs;
- scan structured fields and rendered strings after serialization;
- fail closed or mark evidence `redaction_failed`; never silently persist the
  original value after a redaction error.

Current key/pattern redaction is a useful prototype guard, not proof that PHI is
absent. Free text, novel extensions, encoded values, images, and attachments
require conservative exclusion or specialized inspection.

Logs and traces contain identifiers and bounded metadata only: tenant/run/rule
opaque IDs, status category, duration, byte counts, and approved endpoint host
classification. Never log payloads, tokens, cookies, connection strings,
wrapped DEKs, prompt content, or model responses. Metric labels must be a fixed
low-cardinality allowlist and must not contain tenant names or target URLs.

## Retention baseline

These are recommended defaults, configurable downward:

| Data | Default | Notes |
|---|---:|---|
| Raw target bodies | Not stored | In-memory only, bounded and cleared after the rule. |
| Redacted evidence and final reports | 90 days | Organization may shorten; legal hold is explicit and audited. |
| Partial/failed-run evidence | 30 days | Never promoted to a final report silently. |
| Operational application logs | 30 days | Redacted at source. |
| Security/audit events | 365 days | Export to operator-controlled immutable sink where required. |
| Queue payloads and dead letters | 7 days maximum | Metadata/references only; no secrets or raw FHIR bodies. |
| Encrypted target credentials | Until target deletion | Revoke upstream and cryptographically erase wrapped DEK on deletion. |
| Backups | 35 days | Deletions expire through backup lifecycle; restoration re-applies tombstones. |

Deletion creates an audited tombstone, blocks new reads, deletes primary
objects and indexes, destroys object-specific wrapped DEKs where applicable,
and lets documented backup lifecycle complete. A restore must replay tombstones
before service. Object versions, replicas, exports, caches, search indexes, and
DLQs are part of deletion verification.

Exports require explicit permission, reauthorization, short expiry, an exact
tenant/run scope, audit, and sensitivity markings. Use encrypted transport and
do not place signed URLs in logs. Support access and break-glass access are
time-bound, approved, reasoned, and audited.

## Validation

Test redaction with synthetic values in every relevant FHIR datatype, nested
extensions, narratives, URLs, JWT-like strings, Unicode, base64, oversized and
malformed inputs. Canary secrets and synthetic identifiers must not appear in
logs, traces, queue messages, evidence, reports, or exports. Periodic deletion
tests restore a backup into isolation and verify tombstones and expirations.
