# Production deployment boundary

FHIR Flightcheck `0.1.0` is not production-ready. This document separates the
configuration accepted by the current executable from the controls a real
deployment would still need. Docker Compose is a local synthetic environment,
not a hardened reference architecture.

## Current control-plane runtime contract

The current binary accepts these environment variables:

| Variable | Current behavior |
|---|---|
| `FLIGHTCHECK_LISTEN_ADDR` | Listen address; defaults to `127.0.0.1:8080`. |
| `FLIGHTCHECK_DATABASE_URL` | PostgreSQL URL. Its presence selects durable mode unless storage mode is explicit. |
| `FLIGHTCHECK_STORAGE_MODE` | `postgres` or `memory`. Memory mode is restricted to an explicit loopback listen address. |
| `FLIGHTCHECK_NATS_URL` | Required NATS URL for asynchronous evaluation. |
| `FLIGHTCHECK_JOB_SUBJECT` | Optional; defaults to `flightcheck.jobs.evaluate`. |
| `FLIGHTCHECK_RULE_PACK_DIR` | Optional; defaults to `packages/rule-packs`. |
| `FLIGHTCHECK_API_TOKEN` | Required in durable or non-loopback mode; at least 32 characters. |
| `FLIGHTCHECK_WORKER_TOKEN` | Always required by the control plane; at least 32 characters and distinct from the API token. |
| `FLIGHTCHECK_SIGNING_KEY` | Base64 Ed25519 private key; required in durable mode. |
| `FLIGHTCHECK_ALLOW_LOCAL_DEMO` | `true` allows explicitly flagged private/local target URLs. Never enable for a shared production worker. |

The evaluator worker requires:

| Variable | Current behavior |
|---|---|
| `FLIGHTCHECK_CONTROL_PLANE_URL` | Required absolute control-plane URL. |
| `FLIGHTCHECK_WORKER_TOKEN` | Required bearer token for completion requests. Use the same worker token configured on the control plane, never the API token. |
| `NATS_URL` | NATS URL; defaults to `nats://127.0.0.1:4222`. Set it explicitly outside local development. |
| `FLIGHTCHECK_JOB_SUBJECT` | Optional; defaults to `flightcheck.jobs.evaluate`. |
| `FLIGHTCHECK_DURABLE` | Optional JetStream durable consumer name; defaults to `evaluator`. Give each intended consumer group a deliberate name. |

The web image currently requires `CONTROL_PLANE_URL` and
`FLIGHTCHECK_API_TOKEN` in Compose, but its page reads static demonstration
data and does not use those values for live operations.

Variables named `FLIGHTCHECK_S3_*` appear in `compose.yaml`, but the current Go
service does not read them and the evaluator emits content-addressed
`urn:sha256:` evidence metadata rather than uploading artifact bodies. Do not
infer durable evidence storage from the presence of the Garage container.

## Generate a persistent signing key

Preferred image command:

```console
docker compose run --no-deps --rm control-plane generate-signing-key
```

Current source fallback:

```console
go run ./services/control-plane/cmd/control-plane generate-signing-key
```

Both print a JSON object. Store only its `privateKey` value as
`FLIGHTCHECK_SIGNING_KEY` in a secret manager and distribute the `publicKey`
through a separately authenticated trust process. Never place the private key
in an image, repository, deployment manifest, log, or ticket.

The current runtime imports an exportable private key into process memory. A
production design should replace that with a non-exportable KMS/HSM-backed
Ed25519 signing operation, explicit key states, audit, rotation overlap, and a
revocation/trust policy. That integration is not implemented.

## Minimum deployment topology

If evaluating the prototype in an isolated non-production environment:

1. Run PostgreSQL and NATS JetStream as durable managed or operator-owned
   services with encryption in transit, authentication, backups, and restore
   tests.
2. Run the control plane and evaluator as separate identities and network
   policies. Expose only the control-plane public routes at the edge; keep
   `/internal/v1` reachable only by workers.
3. Inject distinct API and worker tokens from a secret manager. Rotate both by
   deploying consumers and producers in a coordinated maintenance window; the
   current service accepts only one value of each.
4. Inject a persistent signing key from a secret manager and back up its trust
   metadata. Losing it does not alter old signatures, but losing the associated
   public-key record makes verification and provenance harder.
5. Disable `FLIGHTCHECK_ALLOW_LOCAL_DEMO`; restrict worker egress to approved
   target addresses through a proxy or firewall.
6. Put TLS termination, request-size limits, rate limits, access logs with
   redaction, and denial monitoring at the public edge.
7. Pin images by digest, scan them, produce an SBOM, and verify provenance
   according to local supply-chain policy.

This topology reduces exposure but does not close the product limitations
below.

## Controls required before production data

The following are design requirements, not current features:

- OIDC user authentication, authorization, project membership, and auditable
  administrative actions;
- workload identity or short-lived service credentials in place of static
  bearer tokens;
- multi-tenant authorization and PostgreSQL row-level security;
- encrypted target credentials and destination-bound credential delivery;
- real S3-compatible evidence upload, encryption, integrity verification,
  retention, deletion, and backup recovery;
- KMS/HSM-backed report signing and key lifecycle management;
- hardened worker isolation, resource budgets, and separate pools for passive,
  read, and any future write-capable rules;
- DNS/IP revalidation and network-level egress controls sufficient to address
  rebinding and discovered-URL SSRF;
- complete audit export, metrics, traces, alerts, and tested incident runbooks;
- high-availability design, capacity tests, and measured service objectives.

Do not send production PHI, credentials, or clinical write authority to the
current prototype. Review [Limitations](../LIMITATIONS.md), the
[threat model](../security/threat-model.md), and
[data handling](../security/data-handling.md) before any external evaluation.

## Health and rollout checks

```console
curl --fail https://flightcheck.example/healthz
curl --fail https://flightcheck.example/readyz
```

`/healthz` reports process liveness. `/readyz` currently checks repository
availability and signer construction only; it does not prove NATS, worker,
target, or artifact-store readiness. A rollout must also submit and verify a
synthetic canary run through the same queue and worker pool that will serve
traffic.

Rollback the control plane and evaluator together when their contracts or rule
packs change. Preserve PostgreSQL state, the signing-key trust record, and NATS
stream data. Never roll back by replacing a persistent signing key or deleting
an incomplete run.
