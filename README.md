# FHIR Flightcheck

**Production readiness checks for FHIR integrations — evidence-backed, signed, and reproducible.**

[![CI](https://github.com/Arshiamk/fhir-flightcheck/actions/workflows/ci.yml/badge.svg)](https://github.com/Arshiamk/fhir-flightcheck/actions/workflows/ci.yml)
[![Security](https://github.com/Arshiamk/fhir-flightcheck/actions/workflows/security.yml/badge.svg)](https://github.com/Arshiamk/fhir-flightcheck/actions/workflows/security.yml)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev)
[![Python 3.14](https://img.shields.io/badge/Python-3.14-3776AB?logo=python&logoColor=white)](https://www.python.org)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

---

## The problem

Every healthtech engineering team building a FHIR integration faces the same question before go-live:

> *"Is this integration actually safe to put in front of patients?"*

The answer is usually buried in a combination of Postman collections, tribal knowledge, manual checklists, and last-minute security reviews. FHIR Flightcheck replaces that with a **repeatable, automated, evidence-backed readiness assessment** you can run in CI or before every deployment.

---

## What it does

Flightcheck runs **35 deterministic rules** across four packs against your FHIR R4 endpoint, collects cryptographically-addressed evidence for every check, and produces a **signed readiness report** with a clear decision:

| Decision | Meaning |
|---|---|
| `ready` | All rules passed with no blocking findings |
| `conditional` | Low-severity warnings — review before go-live |
| `not_ready` | One or more blocking findings — must fix before go-live |
| `incomplete` | Coverage gap — not enough data to decide |

### Rule packs

| Pack | Rules | What it checks |
|---|---|---|
| **FHIR R4 Conformance** | 9 | Resource structure, required fields, search capabilities, CapabilityStatement |
| **Reliability** | 8 | Error rates, timeout behaviour, pagination, retry-ability |
| **Security & Privacy** | 9 | SMART scopes, TLS, audit logging, PHI exposure in errors |
| **AI Safety & Human Oversight** | 9 | Model authority boundaries, human review gates, prompt injection, capability disclosures |

### Key features

- **Blocker-first reporting** — critical and high findings surface immediately, not buried in a score
- **Content-addressed evidence** — every finding is backed by a `urn:sha256:` hash of the captured payload
- **Ed25519-signed reports** — tamper-evident; verify offline with just the public key
- **Baseline regression gate** — pin a passing report as baseline; future runs fail if new blockers appear
- **SARIF output** — integrates with GitHub Code Scanning, VS Code, and any SARIF-aware tool
- **Idempotent runs** — safe to re-run; same inputs produce the same outcome

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client layer                             │
│   CLI (Go)          Web console (Next.js)                       │
└──────────────┬──────────────────────┬──────────────────────────┘
               │ REST (RFC 9457)       │ Server components
               ▼                       ▼
┌──────────────────────────────────────────────────────────────── ┐
│                    Control plane (Go)                           │
│  Run lifecycle · Ed25519 signing · Baseline management          │
│  URL policy (SSRF) · PHI redaction · Tenant isolation          │
└──────────┬───────────────────────────────────────┬─────────────┘
           │ Transactional outbox                  │ PostgreSQL
           ▼                                       │ (RLS)
┌──────────────────────┐                           │
│   NATS JetStream     │◄──────────────────────────┘
│   (at-least-once)    │
└──────────┬───────────┘
           ▼
┌──────────────────────────────────────────────────────────────── ┐
│                  Evaluator worker (Python)                      │
│  35 rules · Content-addressed evidence · S3 artifact upload    │
│  SARIF generation · AI authority boundary enforcement          │
└──────────────────────────────────────────────────────────────── ┘
           │
           ▼
    Authorized FHIR target (R4)
```

**Polyglot monorepo** — Go for the control plane (correctness, concurrency, single binary), Python for the evaluator (FHIR ecosystem, ML-adjacent tooling), TypeScript for the console (Next.js App Router, server components).

---

## Quick start

**Prerequisites:** Docker Desktop with Compose v2, 8 GB free memory, ports `3000`, `8080`, `8081` available.

```bash
# 1. Clone and configure
git clone https://github.com/Arshiamk/fhir-flightcheck.git
cd fhir-flightcheck
cp .env.example .env

# 2. Generate a signing key and paste privateKey into .env
docker compose run --no-deps --rm control-plane generate-signing-key

# 3. Start the full stack (control plane + NATS + PostgreSQL + HAPI FHIR + web console)
docker compose up --build -d
docker compose ps

# 4. Run your first readiness check against the bundled HAPI FHIR target
docker compose --profile tools run --rm cli \
  init --api http://control-plane:8081 --name "My Flightcheck" \
  --config .flightcheck/config.json

docker compose --profile tools run --rm cli \
  target add --name "Local HAPI" --url http://fhir:8080/fhir \
  --allow-local-demo --config .flightcheck/config.json

docker compose --profile tools run --rm cli \
  run --profile startup-r4 --output .flightcheck/report.json \
  --config .flightcheck/config.json

# 5. Verify the signed report
docker compose --profile tools run --rm cli \
  report verify --config .flightcheck/config.json .flightcheck/report.json
```

**Web console:** http://localhost:3000
**HAPI FHIR:** http://localhost:8080/fhir
**Control plane health:** http://localhost:8081/healthz

---

## Reading a report

```json
{
  "decision": "not_ready",
  "coverage": { "selected": 35, "completed": 35 },
  "findings": [
    {
      "ruleId": "security.smart.scopes",
      "outcome": "fail",
      "severity": "high",
      "summary": "Observed SMART scopes were broader than the rule permits.",
      "remediation": "Advertise and request only resource-specific scopes needed by the workflow.",
      "evidenceRefs": ["urn:sha256:a3f8..."]
    },
    {
      "ruleId": "reliability.timeout.behaviour",
      "outcome": "pass",
      "severity": "medium",
      "summary": "Target returned 408 within 5 s and included Retry-After header."
    }
  ],
  "signature": {
    "algorithm": "Ed25519",
    "keyId": "k_2026_local",
    "value": "base64-encoded-signature"
  }
}
```

- `not_ready` → at least one `high` or `critical` fail
- `conditional` → only `low`/`medium` warnings
- `incomplete` → coverage gap or `inconclusive` result
- `ready` → all rules completed without blocking findings

---

## CI integration

Add a readiness gate to your pipeline. The CLI exits `1` on `not_ready` or `incomplete`:

```yaml
# .github/workflows/integration-check.yml
- name: Run FHIR readiness check
  run: |
    docker compose --profile tools run --rm cli \
      run --profile startup-r4 \
      --baseline .flightcheck/baseline.json \
      --output .flightcheck/report.json \
      --config .flightcheck/config.json
```

Use `--baseline` to gate against a previously-approved report. New blocking findings fail the build; resolved findings are noted in the output.

---

## SARIF output

```bash
# Emit SARIF for GitHub Code Scanning integration
uv run --project workers/evaluator \
  flightcheck-eval evaluate \
  --format sarif \
  --output findings.sarif \
  <manifest.json>
```

Upload `findings.sarif` to GitHub's Code Scanning tab for inline annotations on pull requests.

---

## Security model

- **Separate API and worker tokens** — minimum 32 characters each, distinct secrets
- **URL policy** — private/loopback targets blocked by default (SSRF prevention); per-hop redirect re-validation
- **Ed25519 signatures** — complete reports are signed; verification needs only the public key
- **PHI redaction** — patient identifiers stripped from evidence before storage
- **Tenant isolation** — PostgreSQL RLS enforces organization-scoped data access
- **AI authority boundaries** — the AI rule pack enforces write prohibition, mandatory human review gates, and capability disclosures

See [threat model](docs/security/threat-model.md) and [data handling policy](docs/security/data-handling.md) before pointing Flightcheck at a non-synthetic target.

---

## Tech stack

| Layer | Technology | Why |
|---|---|---|
| Control plane | Go 1.26 | Single binary, strong type system, excellent concurrency |
| Evaluator | Python 3.14 + uv | FHIR ecosystem libraries, async workers |
| Web console | Next.js 15 + TypeScript | App Router server components, zero client JS by default |
| Message queue | NATS JetStream | At-least-once delivery, transactional outbox pattern |
| Database | PostgreSQL + RLS | Tenant isolation, transactional outbox, ACID guarantees |
| Artifact storage | S3-compatible (Garage) | Content-addressed evidence, supply-chain-safe |
| Contracts | JSON Schema + quicktype | Single source of truth across all three runtimes |
| CI | GitHub Actions | CodeQL, Trivy, govulncheck, dependabot, SBOM, pip-audit |

---

## Running the tests

```bash
# Go (control plane + CLI)
go test -race ./...

# Python (evaluator)
uv run --project workers/evaluator pytest workers/evaluator/tests \
  --cov=flightcheck_evaluator --cov-fail-under=90

# TypeScript (contracts + web)
pnpm install --frozen-lockfile
pnpm check
pnpm build:web
```

---

## Project status

`v0.1.0` prototype — the core API, CLI, queue, evaluator, rule packs, signing, and baseline paths are all present and tested. Production-hardening (OIDC, KMS, real artifact encryption, packaged release verification) is tracked in the [roadmap](docs/ROADMAP.md).

---

## Related

- **[HealthGuard](https://github.com/Arshiamk/healthguard)** — clinical AI guardrails SDK: PHI redaction, hallucination detection, prompt injection blocking, and audit trails for LLMs in healthcare. The AI Safety rule pack in Flightcheck evaluates AI systems against the HealthGuard policy surface.

---

## Contributions

v1 maintains a single-author history. Issues and design feedback are welcome; external code PRs are not merged during this phase. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

---

## License

[Apache 2.0](LICENSE)

---

## Standards references

- [HL7 FHIR R4 (4.0.1)](https://hl7.org/fhir/R4/) · [SMART App Launch 2.2](https://hl7.org/fhir/smart-app-launch/) · [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) · [Ed25519 RFC 8032](https://www.rfc-editor.org/rfc/rfc8032) · [JSON Schema 2020-12](https://json-schema.org/draft/2020-12)

> **Assurance boundary:** Flightcheck is an engineering assessment tool, not a certification. A `ready` decision means the selected, versioned rules completed without a blocking result for the observed target and inputs. It does not establish HIPAA, FDA, SOC 2, ISO, clinical-safety, or medical device compliance.
