# ADR 0003: At-least-once jobs with idempotent completion

**Status:** Accepted  
**Date:** 2026-08-18

## Context

Exactly-once execution cannot be guaranteed across a database, message broker,
FHIR target, worker, and artifact store. Claiming otherwise would hide failure
modes. Lost checks are less acceptable than safe duplicate attempts.

## Decision

Use a PostgreSQL transactional outbox and NATS JetStream at-least-once delivery.
The logical execution key is `(run_id, rule_id, rule_version)`. Attempts have
separate identifiers, bounded deadlines, and leases. Workers may repeat passive
or active-read checks. Completion is accepted once through a unique database
constraint; duplicate completion is acknowledged without altering the result.

Active-write checks are outside the default profiles and require a
rule-specific idempotency strategy plus explicit authorization.

## Consequences

- Worker crashes and broker redelivery are recoverable.
- Rule authors must make side effects explicit.
- Observability distinguishes logical executions from attempts.
- Retry policy is based on classified transient failures, never on findings.
