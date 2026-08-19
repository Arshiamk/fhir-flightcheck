# Authoring a rule

Rules are versioned, capability-declared evaluators. They must be safe to retry,
bounded, deterministic unless explicitly marked otherwise, and honest when
evidence is unavailable.

## Definition

Create a definition in `packages/rule-packs` that validates against the
canonical `Rule` schema. A rule declares:

- stable reverse-domain-style identifier;
- semantic version;
- category and default severity;
- passive, active-read, or active-write behavior;
- required capabilities;
- supported FHIR versions;
- deadline;
- standards references; and
- actionable remediation.

Changing evidence interpretation or pass/fail semantics requires a rule version
change. Never recycle an identifier for a different check.

## Capability policy

`network`, `target-credentials`, `fixtures`, `model`, and `write` are explicit
grants. A worker rejects undeclared access. Active-write rules are not included
in default profiles and require a separate idempotency design.

## Result discipline

- `pass`: evidence positively proves the condition.
- `fail`: evidence positively proves a violation.
- `warning`: demonstrated risk tolerated by the selected policy.
- `not_applicable`: the rule cannot apply, with a recorded reason.
- `inconclusive`: required evidence is absent or ambiguous.
- `platform_error`: Flightcheck failed to execute the rule correctly.

Do not return `pass` merely because an endpoint, header, resource, or model is
unavailable.

## Test requirements

Every first-party rule includes:

1. a healthy synthetic case;
2. a failing case with exact expected evidence;
3. unavailable/malformed evidence behavior;
4. redaction assertions;
5. timeout and retry classification where networked; and
6. stable golden output without wall-clock or random values.

Property tests are required for parsers and collection traversal. Network tests
use controlled transports; CI never probes an unowned external clinical system.

## Review checklist

- Is the tested condition narrow and independently understandable?
- Is the standard reference applicable to the pinned version?
- Can the check cause writes or excessive traffic?
- Does evidence contain identifiers, tokens, or free text needing redaction?
- Can duplicate delivery change the result?
- Is severity proportional to demonstrated impact?
- Does remediation tell an engineer exactly what to change?
- Would an unavailable dependency become `inconclusive`, not `pass`?
