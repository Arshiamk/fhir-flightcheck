# ADR 0002: JSON Schema is the cross-language contract source

**Status:** Accepted  
**Date:** 2026-08-18

## Context

The control plane, evaluator, web console, CLI, persisted artifacts, and CI
integration use three languages. Hand-maintained duplicate models would drift
and make old evidence impossible to interpret reliably.

## Decision

JSON Schema 2020-12 under `packages/contracts/schema` is canonical. Go, Python,
and TypeScript artifacts are generated and committed for review. CI recompiles
the schema, rejects generated drift, and validates examples.

Every persisted top-level object carries `schemaVersion`. Additive optional
fields are backward compatible within a major version. Removing fields,
tightening accepted values, changing decision semantics, or changing canonical
serialization requires a new major schema version and migration guidance.

## Consequences

- Cross-language changes begin with one reviewed schema.
- Generated files are not edited by hand.
- Services still validate untrusted messages at runtime; generated static types
  are not a security boundary.
- Reports pin schema and rule versions so historical evidence remains
  interpretable.
