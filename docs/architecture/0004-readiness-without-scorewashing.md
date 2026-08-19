# ADR 0004: Policy decisions without an aggregate compliance score

**Status:** Accepted  
**Date:** 2026-08-18

## Context

A percentage can hide a critical authorization or patient-safety failure behind
many low-value passing checks. It can also be mistaken for regulatory
certification.

## Decision

Report coverage and outcomes per evaluation pack. A versioned policy maps
explicit blocker rules and tolerated warnings to one of:

- `ready`: all required rules completed and no blocker failed;
- `conditional`: no critical blocker failed, but reviewed warnings remain;
- `not_ready`: a configured blocker failed; or
- `incomplete`: selected evidence is missing or a platform error prevented a
  final decision.

The UI places blockers and missing evidence before passing checks. It does not
display a single overall percentage or use compliance/certification language.

## Consequences

- Decisions remain explainable and reviewable.
- Policy changes are visible and versioned.
- Badges link to verifiable reports and include the policy version.
- Users cannot compare unrelated profiles through a misleading scalar score.
