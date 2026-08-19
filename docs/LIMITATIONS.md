# Limitations

FHIR Flightcheck `0.1.0` is a prototype for synthetic engineering evaluation.
The boundaries below are part of the product contract, not fine print.

## Assurance

- Flightcheck does not certify HIPAA, FDA, SOC 2, ISO, legal compliance,
  clinical safety, medical-device status, or fitness for production.
- A `ready` result applies only to the selected rule versions, target snapshot,
  profile, inputs, and observation time. It is not a claim about untested
  behavior or future operation.
- Rule severity and the current fixed decision policy are engineering choices.
  They have not been validated by a regulator or accredited assessor.
- The rule packs currently have no populated `standardReferences` mappings.
  Repository documentation references relevant standards, but finding-level
  normative traceability remains incomplete.

## FHIR and SMART scope

- Only FHIR R4 `4.0.1` and the single `startup-r4` profile are accepted.
- The 35 first-party rules are targeted observations, not a complete FHIR
  validator or implementation-guide conformance suite.
- The prototype does not invoke the official HL7 validator, resolve pinned
  implementation-guide packages, or perform complete terminology validation.
- Five rules perform active network reads; the remaining rules depend on
  synthetic fixture assertions. No default rule performs a clinical write.
- SMART checks observe discovery metadata and advertised scopes only. They do
  not execute a complete authorization-code launch, token exchange, refresh,
  revocation, or patient-context workflow.

## Current end-to-end demo

- The control plane queues an empty fixture object. The files under
  `fixtures/synthea/` are used by evaluator tests and one-shot evaluation, but
  cannot currently be selected through the public API or Go CLI.
- Fixture-dependent rules therefore do not receive the bundled healthy or
  broken scenarios in a Compose run and may return `inconclusive`. A completed
  signed report proves pipeline completion, not full scenario coverage.
- The bundled HAPI server is a generic local FHIR R4 target. It is not a
  complete SMART server or a representative production EHR.
- The Next.js console reads static synthetic demonstration data. Buttons that
  imply run creation, export, target management, policy inspection, or audit
  navigation are not connected to the control plane.
- No screenshots are committed.

## Evidence and reports

- The worker creates evidence metadata and content hashes with `urn:sha256:`
  references. It does not upload evidence bodies to the Garage/S3 service.
- `FLIGHTCHECK_S3_*` variables in Compose are not consumed by the current Go
  control plane. The Garage container does not establish artifact durability.
- Reports sign the report contract after full finding coverage. They do not
  sign or prove possession of external artifact bodies.
- The CLI verifier trusts the public key in its local config or supplied by
  `--key`. It does not implement a certificate chain, transparency log,
  revocation list, trust-on-first-use warning, or managed trust store.
- There is no online signing-key rotation overlap. The service accepts one
  exportable base64 Ed25519 private key.
- There is no SARIF output, HTML/PDF export, or `report open` command in the
  current CLI.

## Identity, tenancy, and secrets

- The executable has one fixed organization identifier (`org_local`) and no
  user identity, OIDC, sessions, RBAC, project membership, or administrative
  authorization.
- Authentication uses one static API bearer token and one static worker bearer
  token. There is no expiry, scope, overlap rotation, or workload identity.
- PostgreSQL rows include organization fields, but production-grade row-level
  security and tenant-scoped database roles are not implemented.
- Credential references are validated strings and are removed from run
  manifests. There is no credential broker, envelope encryption, KMS/HSM
  integration, or target-authentication flow.
- The Compose web service receives the API token server-side, but the current
  static console does not use it.

## Network and worker isolation

- The URL policy blocks disallowed address classes and private targets unless
  explicitly overridden, but application validation alone is not a complete
  SSRF defense. Production requires controlled DNS resolution and network-level
  egress enforcement.
- The HTTP evaluator does not follow redirects, which reduces exposure but also
  means redirect behavior is not fully assessed.
- `--allow-local-demo` and `FLIGHTCHECK_ALLOW_LOCAL_DEMO=true` are local
  exceptions. Compose enables the server-side exception for its synthetic
  network; do not carry it into a shared deployment.
- Rules run in one Python worker process. There is no per-rule container or
  microVM sandbox, CPU/memory quota, filesystem isolation, separate capability
  pool, or worker quarantine control in the executable.
- The startup catalog rejects rules requesting target credentials or write
  capability. Community rule admission and signature verification are not
  implemented.

## Reliability and operations

- PostgreSQL and JetStream provide a durable local state and dispatch path, but
  no published load, soak, chaos, failover, or recovery benchmark exists.
- The current worker's completed-job cache is in memory. PostgreSQL completion
  idempotency is the durable authority after restart.
- Exhausted or malformed JetStream messages are terminated; there is no
  operator-facing dead-letter queue workflow in the current implementation.
- `/readyz` checks the repository and signer object, not NATS, worker
  availability, object storage, or end-to-end run completion.
- The CLI waits two minutes for a run and has no configurable wait timeout,
  cancellation, resume, or live progress.
- Backups, point-in-time recovery, artifact recovery, high availability,
  autoscaling, quotas, rate limiting, and disaster recovery are operator
  responsibilities and are not supplied as a production distribution.
- Observability is limited to structured Go HTTP/dispatcher logs and included
  libraries. End-to-end OpenTelemetry traces, Prometheus metrics, dashboards,
  alerting, and audit export are not wired.

## Platform and release

- Docker Compose is the only assembled deployment. There is no supported Helm
  chart or Kubernetes production package.
- The release workflow may evolve; do not assume every image or binary has a
  published signature, SBOM, provenance attestation, or compatibility policy
  without checking the specific release assets.
- Upstream Compose images use pinned tags, not immutable digests.
- The tested development toolchains are those declared in CI. Other versions
  and architectures may work but are not established by documentation alone.
- No performance claims are made. Use the
  [synthetic benchmark methodology](benchmarks/README.md) and publish raw data
  before quoting results.

## Data safety

- The default intent is synthetic data, but configuration does not technically
  prevent a user from targeting a real endpoint.
- Prototype redaction cannot prove that PHI or secrets are absent from every
  nested, encoded, free-text, image, attachment, or extension value.
- Do not provide production PHI, persistent target credentials, or clinical
  write authority to this version.

Track planned closure of these gaps in the [roadmap](ROADMAP.md). A roadmap item
is not an implemented control until code, tests, operations guidance, and a
release artifact demonstrate it.
