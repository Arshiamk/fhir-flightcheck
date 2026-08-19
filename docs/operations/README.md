# Operations guide

This guide covers the current Docker Compose prototype. Commands assume the
repository root and a populated `.env`. For production design requirements,
see [production deployment](../deployment/production.md).

## Service inventory

- `control-plane`: public API, run state, readiness decisions, and report
  signing.
- `evaluator`: JetStream consumer and deterministic rule execution.
- `postgres`: projects, targets, runs, findings, reports, jobs, idempotency,
  and transactional outbox.
- `nats`: `FLIGHTCHECK_JOBS_V1` JetStream work stream.
- `fhir`: local HAPI FHIR R4 target.
- `garage`: local object-store process; not used by the current application
  path for artifact bodies.
- `web`: static synthetic operations-console demonstration.
- `cli`: disposable tool-profile container.

## Start, inspect, and stop

```console
docker compose up --build -d
docker compose ps
docker compose logs --no-color --since=10m control-plane evaluator postgres nats
docker compose stop
docker compose down
```

Use `docker compose down --volumes` only for an intentional local reset. It
permanently removes PostgreSQL, JetStream, and Garage volumes.

## Health checks

```console
curl --fail http://localhost:8081/healthz
curl --fail http://localhost:8081/readyz
curl --fail http://localhost:8080/fhir/metadata
docker compose exec postgres pg_isready -U flightcheck -d flightcheck
docker compose exec nats wget -q -O - http://localhost:8222/healthz
```

Interpret these narrowly:

- `/healthz` means the control-plane HTTP handler is alive.
- `/readyz` pings its configured repository and checks that the service and
  signer exist.
- HAPI metadata proves the synthetic target can answer its capability endpoint.
- PostgreSQL and NATS checks prove those individual dependencies respond.

None proves that a worker consumed a job. The end-to-end canary is a complete
CLI run followed by offline report verification:

```console
docker compose --profile tools run --rm cli run --profile startup-r4 --output .flightcheck/canary-report.json --config .flightcheck/config.json
docker compose --profile tools run --rm cli report verify --config .flightcheck/config.json .flightcheck/canary-report.json
```

An `incomplete` or `not_ready` canary can still prove message flow and signing,
but it is not a successful target-readiness result. Record and investigate its
findings.

## Authentication failures

Public API routes in durable Compose mode require
`Authorization: Bearer $FLIGHTCHECK_API_TOKEN`. Worker completion routes require
the distinct `FLIGHTCHECK_WORKER_TOKEN`. Health routes require neither.

For repeated HTTP 401 responses:

1. Confirm the caller is using the correct token class; never give the worker
   token to CLI or browser users.
2. Confirm the token values are at least 32 characters and distinct.
3. Recreate the affected containers after changing `.env`:

   ```console
   docker compose up -d --force-recreate control-plane evaluator web
   ```

4. Inspect redacted status logs. Do not print token values:

   ```console
   docker compose logs --no-color --since=10m control-plane evaluator
   ```

The current process supports one API token and one worker token, with no
overlap set. Rotation therefore requires a coordinated maintenance window.

## Queue backlog and stuck runs

A run remains `queued` until the outbox is published and a worker completes the
job. The CLI waits for at most two minutes.

```console
docker compose ps control-plane evaluator nats postgres
docker compose logs --no-color --since=15m control-plane evaluator nats
docker compose restart evaluator
```

Look for `outbox dispatch failed`, `outbox publish deferred`, NATS connection
errors, worker configuration errors, and completion HTTP status codes. The
dispatcher retries failed publication with bounded exponential delay. The
worker fetches up to eight messages and terminates malformed or exhausted
messages after at most the job's configured attempts.

The current worker has an in-memory completion cache only. PostgreSQL remains
the authoritative completion boundary, so a worker restart can cause safe
redelivery but must not create a second report for the same completion.

Do not delete queue or database rows to unstick a run. Preserve logs and state
for diagnosis; start a new run after restoring the dependency.

## Report signing

Durable mode cannot start without `FLIGHTCHECK_SIGNING_KEY`. If it is absent in
non-durable memory mode, the service generates an ephemeral key and warns that
reports cannot be verified after restart.

Verify a saved report with the public key captured by `flightcheck init`:

```console
docker compose --profile tools run --rm cli report verify --config .flightcheck/config.json .flightcheck/report.json
```

Exit code `4` means decoding, contract validation, key parsing, or signature
verification failed. Preserve the report unchanged and compare its `keyId`
with the trusted key record.

The current service has no online multi-key rotation. To replace a compromised
or expiring key:

1. Stop new runs.
2. Preserve the old public key and key ID in the operator trust record.
3. Generate a new key using the documented deployment command.
4. update the secret manager and recreate the control plane;
5. re-run `flightcheck init` into a new config file to capture the new public
   key for future reports;
6. verify one synthetic report with the new config and an old report with the
   archived old public key using `report verify --key OLD_PUBLIC_KEY REPORT.json`.

Changing the key does not re-sign old reports. Never discard an old public key
while its reports must remain verifiable.

## PostgreSQL backup and restore

For the local stack, create a logical backup without exposing the password on
the command line:

```console
docker compose exec -T postgres pg_dump -U flightcheck -d flightcheck -Fc > flightcheck.dump
```

Treat the dump as sensitive: it contains target URLs, findings, evidence
metadata, and operational state. Store it encrypted and outside the repository.
Test restoration into an isolated disposable database before relying on it.
Provider-native physical backups, point-in-time recovery, retention, and
restore verification are required outside local development.

## Evidence and data handling

The current worker creates evidence metadata with content hashes and
`urn:sha256:` storage URIs. It does not upload artifact bodies to Garage, even
though Compose starts that service. Do not report object-store durability,
retention, or recovery as operationally verified.

Logs should contain opaque identifiers, status, and redacted errors only.
Never paste `.env`, report private keys, database URLs, raw FHIR resources, or
tokens into incident channels. Follow
[data handling](../security/data-handling.md) for classification guidance,
while noting that several listed controls remain design targets.

## Incident escalation

Stop evaluation and preserve evidence when any of these occurs:

- suspected cross-organization access or wrong-target evaluation;
- a token, signing key, credential, or patient identifier appears in output;
- a report verifies under an unexpected key;
- a worker reaches an unapproved private or link-local destination;
- repeated completion conflicts or unexplained report changes.

The repository's detailed runbooks cover
[queue backlog](../runbooks/queue-backlog.md),
[worker quarantine](../runbooks/worker-quarantine.md),
[signing-key rotation](../runbooks/key-rotation.md),
[artifact recovery](../runbooks/artifact-recovery.md), and
[cross-tenant suspicion](../runbooks/cross-tenant-suspicion.md). Some steps in
those runbooks describe the intended production platform; verify that a named
control exists before depending on it.
