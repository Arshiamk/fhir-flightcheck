# Service-level objectives

These are initial production targets, not measurements or contractual promises.
They apply only after telemetry and an on-call owner exist. Local/demo
deployments are excluded.

| Service level indicator | Objective | Window and measurement |
|---|---:|---|
| Control-plane availability | 99.9% | Rolling 30 days; successful, authorized requests excluding documented maintenance, client 4xx, and requests rejected by rate/policy limits, divided by eligible requests. |
| Accepted-to-start latency | 99% under 60 seconds | Rolling 30 days; time from committed run acceptance to first rule attempt, for admitted runs while within published tenant quota. |
| Report completion | 99% under 15 minutes | Rolling 30 days; admitted standard-profile synthetic/sandbox runs that reach a terminal complete or explicitly partial state. Target-caused timeouts are classified separately; platform errors count against the SLO. |
| Evidence durability | 99.999999999% annual target | Provider durability target plus Flightcheck verification/backup controls; measured operationally by successful scheduled inventory, digest verification, replication and restore tests. This is not independently achieved by hashing. |

## Measurement rules

Use server-side monotonic durations and immutable state-transition timestamps.
Do not exclude dependency failures under Flightcheck's control, deploy failures,
worker crashes, queue delay, storage errors, or accidental tenant throttling.
Do exclude operator-declared maintenance announced before the window, clearly
invalid/unauthorized requests, user cancellation, and target failure only from
the report-latency SLI; those outcomes still receive separate metrics.

Segment dashboards by region, deployment, profile, and synthetic-versus-remote,
using bounded labels. Never label metrics with tenant names, target URLs, or
patient data. Publish sample count and “no data” explicitly; low volume is not
100% reliability.

## Error budgets and alerts

For 99.9% availability the 30-day budget is about 43.2 minutes. Page on
multi-window, multi-burn alerts rather than a single transient breach:

- fast: 14.4x burn over 1 hour, confirmed over 5 minutes;
- slow: 6x burn over 6 hours, confirmed over 30 minutes;
- queue page: oldest eligible work exceeds 5 minutes for 10 minutes or DLQ
  grows unexpectedly;
- evidence page: any confirmed digest mismatch, cross-tenant access, signing
  failure for a final report, or failed scheduled restore.

When more than 50% of a monthly budget is consumed, pause risky launches and
assign corrective work. At 100%, changes are limited to incident remediation
and reliability/security improvements until an owner approves recovery.

## Supporting indicators

Track p50/p95/p99 API and rule latency, queue oldest age and depth by bounded
worker class, retries, redeliveries, DLQ rate, worker OOM/timeouts, redaction
failure, artifact upload/read/digest failure, final versus partial reports,
signing failures, authorization denials, and tenant-fairness saturation.

Evidence durability is validated through daily inventory/digest sampling,
provider replication alarms, quarterly isolated restore exercises, and annual
key/backup disaster recovery. A successful object-store API call alone is not
durability evidence.
