# Synthetic benchmark methodology

No performance result is published for FHIR Flightcheck `0.1.0`. This document
defines a reproducible way to measure the current prototype without presenting
unrun targets, estimates, or local observations as product claims.

Benchmarks use only the bundled HAPI FHIR server and synthetic/empty fixture
path. They do not use production PHI, paid model APIs, or third-party FHIR
servers. Results describe one tested commit, host, and configuration; they do
not establish a service-level objective or capacity guarantee.

## Questions measured

1. **End-to-end run latency:** wall-clock time from CLI run creation until a
   signed report is returned.
2. **Throughput under controlled concurrency:** completed runs per minute at a
   declared number of simultaneous CLI callers.
3. **Failure behavior:** completion rate, timeout count, report decision, and
   signature-verification count under the same load.
4. **Resource envelope:** peak/average CPU and memory per container during the
   measurement window.

Do not combine these into one score. Report latency distributions, errors,
coverage, and resource use separately.

## Pin the test inputs

Record all of the following with the result:

- exact Git commit (`git rev-parse HEAD`) and whether the tree was clean;
- operating system, CPU model/core allocation, RAM, storage type, and Docker
  Desktop/Engine resource limits;
- Docker and Compose versions;
- image IDs from `docker compose images`;
- PostgreSQL, NATS, Garage, HAPI, Go, Python, Node, pnpm, and uv versions;
- rule-pack names, versions, and the total selected rule count;
- profile (`startup-r4`), target URL, and concurrency;
- whether images and dependency caches were warm;
- warm-up count, measured sample count, timeout, and measurement tool;
- any non-default environment setting, while omitting secret values.

The Compose file pins upstream image tags but not immutable image digests.
Capture image IDs for every published run so a moved tag cannot silently alter
the test.

## Prepare the stack

Follow [local deployment](../deployment/local.md), including token and signing
key setup. Then:

```console
docker compose down --volumes
docker compose up --build -d
docker compose ps
curl --fail http://localhost:8081/readyz
curl --fail http://localhost:8080/fhir/metadata
docker compose --profile tools run --rm cli init --api http://control-plane:8081 --name "Benchmark" --config .flightcheck/config.json
docker compose --profile tools run --rm cli target add --name "Local HAPI" --url http://fhir:8080/fhir --allow-local-demo --config .flightcheck/config.json
```

Resetting volumes makes the initial state reproducible but destroys local data.
Never run that reset against a shared environment.

The current queued-run contract sends an empty fixture object. The repository's
`fixtures/synthea/*.json` files are exercised by evaluator tests but are not
selectable through the end-to-end API. State this limitation in every result;
do not label the end-to-end benchmark as full-fixture evaluation.

## End-to-end latency protocol

Use five unrecorded warm-up runs, followed by at least 30 measured sequential
runs. Each run writes a different report file and is verified offline. Keep
the stack running between samples.

PowerShell measurement example:

```powershell
1..5 | ForEach-Object {
  docker compose --profile tools run --rm cli run --profile startup-r4 --config .flightcheck/config.json | Out-Null
}

"sample,elapsed_ms,run_exit,verify_exit" | Set-Content .flightcheck/benchmark.csv
1..30 | ForEach-Object {
  $report = ".flightcheck/benchmark-report-$_.json"
  $watch = [System.Diagnostics.Stopwatch]::StartNew()
  docker compose --profile tools run --rm cli run --profile startup-r4 --output $report --config .flightcheck/config.json | Out-Null
  $runExit = $LASTEXITCODE
  $watch.Stop()
  docker compose --profile tools run --rm cli report verify --config .flightcheck/config.json $report | Out-Null
  $verifyExit = $LASTEXITCODE
  "$_,$($watch.ElapsedMilliseconds),$runExit,$verifyExit" | Add-Content .flightcheck/benchmark.csv
}
```

This measurement includes startup of the disposable CLI container, API
round-trips, queue delay, evaluation, report creation, and report retrieval. It
does not include image builds or the separate offline verification time.

For a native CLI measurement, build one pinned binary and replace the Compose
CLI invocation. Report that result as a different benchmark because it excludes
container startup.

## Controlled-concurrency protocol

Run separate experiments at concurrency `1`, `2`, `4`, and `8`. Use a fresh
stack for each level, perform five warm-ups, then submit at least 30 total runs.
Keep total submissions constant and start callers from the same host. Do not
raise concurrency after errors merely to obtain a higher number.

The evaluator pulls messages in batches of eight and uses one Compose worker
process. This implementation detail must accompany results; it is not evidence
that eight runs execute with isolated CPU or memory budgets.

For every level, record:

- submitted, completed, timed out, and failed command counts;
- verified and unverifiable report counts;
- `ready`, `conditional`, `not_ready`, and `incomplete` counts;
- p50, p90, p95, p99, minimum, and maximum wall-clock latency;
- total measured window and completed runs per minute;
- report coverage range;
- container CPU and memory samples.

Use a statistics script that reads the raw CSV; retain both the raw samples and
the script with any published result. Do not calculate percentiles from rounded
chart values.

## Resource sampling

Capture container usage throughout the measured window:

```console
docker stats --no-stream
docker compose logs --no-color --since=30m control-plane evaluator postgres nats
```

`docker stats --no-stream` is a point sample, not a peak. For publishable
resource claims, sample at a fixed interval of one second or use an external
container metrics collector, then describe the collector and aggregation.
Check logs for retries and errors before accepting a run.

## Validity and publication rules

Reject or clearly mark a benchmark when:

- any report fails offline signature verification;
- the control plane, worker, database, or broker restarted unexpectedly;
- a measured CLI command timed out at its current two-minute limit;
- background workloads or Docker resource limits changed during the run;
- the rule count, profile, target, or image IDs differ across compared runs;
- raw samples or environment metadata are missing.

Publish results as a dated artifact with the exact commit and method. Include
all failures and incomplete reports. Compare commits only on the same host and
configuration, or label the comparison as non-controlled. Never extrapolate
single-host Compose results into production capacity, multi-tenant isolation,
or clinical workload claims.

## Results template

Use this structure after the protocol has actually been run:

```text
Date (UTC):
Commit / clean tree:
Host and Docker allocation:
Image IDs:
Profile / target / selected rules:
Fixture path limitation:
Warm-ups / measured samples / concurrency:
Measurement command and timeout:
Completion and verification counts:
Decision and coverage counts:
Latency min / p50 / p90 / p95 / p99 / max:
Measured window / runs per minute:
CPU and memory collection method:
Retries, restarts, and anomalies:
Raw-data and analysis-script location:
```

Blank fields mean “not measured,” not zero.
