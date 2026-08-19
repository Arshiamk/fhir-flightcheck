# Queue backlog

**Triggers:** oldest eligible work over 5 minutes for 10 minutes, sustained
depth growth, redelivery spike, or DLQ growth. Do not paste queue payloads into
issues or chat.

1. Declare an incident, record UTC start, deployment, affected worker class and
   tenant-safe run IDs. Check control-plane acceptance, JetStream health,
   consumer lag, worker readiness/saturation, dependency latency, retry reasons,
   and recent deploy/config changes.
2. Stop amplification: pause nonessential schedules and low-priority admission;
   cap retries; disable the offending rule/version or target only with audited
   scope. Preserve existing messages. Do not purge or broadly replay.
3. If workers are healthy but saturated, scale only after confirming database,
   target, storage and egress capacity. Preserve per-tenant fairness. If a
   poison message/rule exists, quarantine its exact digest/attempt and move it
   through the worker-quarantine process.
4. Drain in bounded batches. Replays require original manifest digest, current
   authorization, a new attempt identity, deduplication, and safe target-side
   idempotency. Expired/cancelled work is marked terminal, not executed.
5. Verify oldest age and DLQ return to baseline, accepted runs complete, no
   duplicate findings/evidence exist, and SLO impact is recorded.

Escalate immediately for queue credential misuse, cross-tenant subjects, forged
envelopes, or unexpected active-write work. Retain relevant audit, metrics,
message metadata, consumer state and deploy digests under incident policy;
redact all captures.
