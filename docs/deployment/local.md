# Local deployment

The local stack is a synthetic evaluation laboratory. It is not configured for
production data. Docker Compose starts PostgreSQL, NATS JetStream, Garage,
HAPI FHIR R4, the control plane, an evaluator worker, and the static
operations-console demonstration.

## Prerequisites

- Docker Desktop with Compose v2-compatible commands
- at least 8 GB of free memory
- ports 3000, 3900, 8080, and 8081 available on loopback

## Start

Create the local environment file:

```console
copy .env.example .env
```

On macOS or Linux, use `cp .env.example .env`.

Replace every placeholder before starting:

- `FLIGHTCHECK_API_TOKEN`: at least 32 characters;
- `FLIGHTCHECK_WORKER_TOKEN`: a different value, at least 32 characters;
- `FLIGHTCHECK_SIGNING_KEY`: a base64 Ed25519 private key generated below;
- `POSTGRES_PASSWORD` and `S3_SECRET_KEY`: random local-only values.

The API token authenticates the CLI and web service to public API routes. The
worker token authenticates evaluator completion requests to `/internal/v1`.
They are deliberately separate and the control plane refuses to start if they
are equal.

## Generate the signing key

The control-plane image contains the current key generator:

```console
docker compose run --no-deps --rm control-plane generate-signing-key
```

It prints one JSON object with `algorithm`, `privateKey`, `publicKey`, and
`keyId`. Copy only `privateKey` into `FLIGHTCHECK_SIGNING_KEY` in `.env`; keep
the complete output out of source control, shell history, logs, and tickets.

This image command is the preferred path but requires a running Docker engine.
The current source-level fallback, while packaging is still in progress, is:

```console
go run ./services/control-plane/cmd/control-plane generate-signing-key
```

The fallback must run from the repository root with Go 1.26.6 available. It
executes the same generator and has the same output-handling requirements.

Start the services:

```console
docker compose up --build -d
docker compose ps
```

Compose binds user-facing ports to `127.0.0.1`; containers communicate on the
private project network.

Once healthy:

- console: `http://localhost:3000`
- control-plane health: `http://localhost:8081/healthz`
- synthetic HAPI FHIR R4: `http://localhost:8080/fhir`

## Run the tool-profile CLI

The `cli` service is behind the Compose `tools` profile. It is disposable and
only `.flightcheck/` is mounted, so every command must use the persisted config
path shown here:

```console
docker compose --profile tools run --rm cli init --api http://control-plane:8081 --name "Local Flightcheck" --config .flightcheck/config.json
docker compose --profile tools run --rm cli target add --name "Local HAPI" --url http://fhir:8080/fhir --allow-local-demo --config .flightcheck/config.json
docker compose --profile tools run --rm cli run --profile startup-r4 --output .flightcheck/report.json --config .flightcheck/config.json
docker compose --profile tools run --rm cli report verify --config .flightcheck/config.json .flightcheck/report.json
```

The URL uses the Compose service name because the CLI runs inside the project
network. `--allow-local-demo` is required for the private container address.
Do not use that flag for an untrusted target.

The queued-run path currently sends an empty fixture object. Rules that require
the repository's files under `fixtures/synthea/` are therefore not exercised
with those files through this workflow and may be `inconclusive`. The evaluator
test suite and one-shot evaluator CLI cover those fixtures separately; wiring
fixture selection into the control plane is on the roadmap.

## Verify the stack

```console
docker compose config --quiet
curl --fail http://localhost:8081/healthz
curl --fail http://localhost:8081/readyz
curl --fail http://localhost:8080/fhir/metadata
docker compose --profile tools run --rm cli help
docker compose logs --no-color --since=5m control-plane evaluator
```

On PowerShell, `Invoke-WebRequest -UseBasicParsing <URL>` can replace `curl`.
`/healthz` proves that the HTTP process is alive. `/readyz` also pings the
configured repository; it does not currently test NATS, the worker, or Garage.

## Reset

```console
docker compose down
docker compose down --volumes
```

The second command permanently removes local run state and artifacts.

## Production boundary

Compose is not a production topology. The current executable uses static bearer
tokens, one fixed local organization, a raw base64 signing key, and no live
object-store artifact upload. Production use requires work beyond replacing
Compose secrets. See [production deployment](production.md) for the exact
current runtime contract and the controls that remain prerequisites rather
than implemented features.
