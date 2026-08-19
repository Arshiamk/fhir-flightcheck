# FHIR Flightcheck

FHIR Flightcheck turns a repeatable set of FHIR R4 integration checks into a
blocker-first readiness report. Run it against the bundled HAPI FHIR target,
inspect the evidence behind each finding, save a signed report, and use a
verified baseline as a regression gate.

> **Assurance boundary:** Flightcheck is an engineering assessment tool, not a
> certification. A `ready` decision means that the selected, versioned rules
> completed without a blocking result for the observed target and inputs. It
> does not establish HIPAA, FDA, SOC 2, ISO, legal, clinical-safety, or medical
> device compliance.

## What is implemented

- A Go control plane and CLI with versioned JSON contracts, idempotent
  mutations, PostgreSQL persistence, and RFC 9457-style problem responses.
- Asynchronous evaluation through a PostgreSQL transactional outbox and NATS
  JetStream, with bounded worker redelivery and idempotent completion.
- 35 deterministic rules in four packs: FHIR R4 conformance (9), reliability
  (8), security/privacy/auditability (9), and AI safety/human oversight (9).
- Explicit outcomes (`pass`, `fail`, `warning`, `not_applicable`,
  `inconclusive`, `platform_error`) and decisions (`ready`, `conditional`,
  `not_ready`, `incomplete`) instead of an aggregate compliance score.
- Ed25519-signed complete reports, offline verification, baseline selection,
  and a CLI regression gate with documented exit codes.
- A Next.js blocker-first operations-console demonstration. It currently uses
  reviewed static synthetic data; it is not yet connected to the control-plane
  API.

See [Limitations](docs/LIMITATIONS.md) for the important gaps between the
current prototype and the intended production platform.

## Architecture

```text
CLI / demo web console
          |
          v
Go control plane ---- PostgreSQL (state + transactional outbox)
          |                         |
          +---- NATS JetStream <----+
                     |
                     v
              Python evaluator ---- authorized FHIR target
                     |
                     v
             findings + evidence metadata
                     |
                     v
             signed readiness report
```

The control plane is the state-transition and signing authority. It creates an
immutable run manifest, commits the run and outbox message together, and
accepts worker completion through a separately authenticated internal route.
The evaluator executes only declared rule capabilities. Complete reports are
signed over canonical report content; the CLI stores the public key during
initialization and can verify a report without contacting the service.

More detail: [architecture overview](docs/architecture/README.md).

## Quick start

Prerequisites: Docker Desktop with Compose v2, at least 8 GB free memory, and
loopback ports `3000`, `3900`, `8080`, and `8081`.

1. Create local configuration and replace every placeholder secret. The API
   and worker tokens must be distinct and at least 32 characters.

   ```console
   copy .env.example .env
   docker compose run --no-deps --rm control-plane generate-signing-key
   ```

   Copy only the command's `privateKey` value into
   `FLIGHTCHECK_SIGNING_KEY` in `.env`. If Docker is unavailable but Go 1.26.6
   is installed, use the fallback documented in
   [local deployment](docs/deployment/local.md#generate-the-signing-key).

2. Start the synthetic stack.

   ```console
   docker compose up --build -d
   docker compose ps
   ```

3. Use the Compose `tools` profile. The explicit config path is required so
   CLI state survives disposable containers.

   ```console
   docker compose --profile tools run --rm cli init --api http://control-plane:8081 --name "Local Flightcheck" --config .flightcheck/config.json
   docker compose --profile tools run --rm cli target add --name "Local HAPI" --url http://fhir:8080/fhir --allow-local-demo --config .flightcheck/config.json
   docker compose --profile tools run --rm cli run --profile startup-r4 --output .flightcheck/report.json --config .flightcheck/config.json
   docker compose --profile tools run --rm cli report verify --config .flightcheck/config.json .flightcheck/report.json
   ```

The console is at <http://localhost:3000>; its current data is explicitly a
static demonstration. The HAPI target is at <http://localhost:8080/fhir>, and
the control-plane health endpoint is at <http://localhost:8081/healthz>.

The current end-to-end path does not inject the repository's synthetic fixture
files into queued runs. Fixture-dependent rules therefore report only what the
empty run fixture permits, commonly `inconclusive`; this is expected prototype
behavior, not a successful readiness assessment.

## Reading a report

Review blockers and missing evidence before passes. This excerpt illustrates
the implemented report contract and decision logic; identifiers and values are
examples, not benchmark results:

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
      "evidenceRefs": ["evidence_example"],
      "remediation": "Advertise and request only resource-specific scopes needed by the workflow."
    }
  ],
  "signature": {
    "algorithm": "Ed25519",
    "keyId": "example-key-id",
    "value": "<base64-signature>"
  }
}
```

A failed critical or high-severity rule yields `not_ready`. Lower-severity
failures or warnings yield `conditional`. Missing coverage, an inconclusive
result, or a platform error yields `incomplete`. A complete report with none of
those conditions is `ready`.

## Verification

Run the same checks used by CI:

```console
pnpm install --frozen-lockfile
pnpm check
pnpm build:web
go vet ./cmd/flightcheck/... ./services/control-plane/...
go test -race ./cmd/flightcheck/... ./services/control-plane/...
uv sync --project workers/evaluator --locked --all-groups
uv run --project workers/evaluator ruff check workers/evaluator
uv run --project workers/evaluator ruff format --check workers/evaluator
uv run --project workers/evaluator mypy workers/evaluator/src
uv run --project workers/evaluator pytest workers/evaluator/tests --cov=flightcheck_evaluator --cov-fail-under=90
docker compose config --quiet
```

Required toolchains are Node.js 24 with pnpm 11.22.0, Go 1.26.6, Python 3.14,
uv, and Docker Compose. For a shorter smoke check, see
[local deployment](docs/deployment/local.md#verify-the-stack).

## Security model

- The API token and worker token are separate bearer-token boundaries and must
  be distinct. Health and readiness endpoints are intentionally unauthenticated.
- Durable or non-loopback control-plane mode requires an API token of at least
  32 characters; every worker deployment requires a non-empty worker token,
  while the control plane enforces 32 characters.
- Durable mode requires a persistent Ed25519 private key. Complete reports are
  signed; incomplete reports are not.
- Target URLs are restricted to HTTP(S). Private and loopback targets require
  the explicit local-demo override; redirects are not followed by the evaluator.
- The startup rule profile excludes target-credential and write capabilities.
  Target credentials are references, not plaintext values in run manifests.
- The current prototype does **not** implement the production OIDC, RBAC,
  PostgreSQL RLS, KMS/HSM, artifact encryption, workload identity, or sandbox
  controls described as design targets in the threat model.

Read the [threat model](docs/security/threat-model.md), [data-handling
policy](docs/security/data-handling.md), and
[production deployment boundary](docs/deployment/production.md) before using a
non-synthetic target.

## Screenshots

No screenshots are committed yet. The operations-console demonstration can be
viewed locally at <http://localhost:3000>. When screenshots are added, they
must be captured from the shipped synthetic demo, contain no real patient data
or credentials, include useful alt text, and match the current release.

## Project status

FHIR Flightcheck is a `0.1.0` prototype. The core local API, CLI, queue,
evaluator, contracts, rule packs, signing, and baseline paths are present.
Production identity, real artifact storage, fixture selection in queued runs,
live console integration, packaged release verification, and published
benchmark results remain work in progress. See the [roadmap](docs/ROADMAP.md).

## Contributions

Version 1 intentionally maintains a single-author commit history. Issues and
design feedback are welcome, but external code commits and pull requests are
not merged during this phase. This policy is enforced by the repository's
authorship check and can be reconsidered after the first stable release.

## License

Licensed under the [Apache License 2.0](LICENSE).

## Standards references

- [HL7 FHIR R4 (4.0.1)](https://hl7.org/fhir/R4/)
- [FHIR R4 security and privacy module](https://hl7.org/fhir/R4/secpriv-module.html)
- [SMART App Launch 2.2](https://hl7.org/fhir/smart-app-launch/)
- [RFC 9457: Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc9457)
- [JSON Schema Draft 2020-12](https://json-schema.org/draft/2020-12)
- [Ed25519, RFC 8032](https://www.rfc-editor.org/rfc/rfc8032)

References describe the standards that inform rules and contracts. They do not
imply endorsement, certification, or complete conformance coverage.
