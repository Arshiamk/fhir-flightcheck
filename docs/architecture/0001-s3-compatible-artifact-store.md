# ADR 0001: Use Garage for local evidence storage

**Status:** Accepted  
**Date:** 2026-08-18

## Context

The approved design named MinIO as the local S3-compatible evidence store.
MinIO Community Edition stopped publishing maintained prebuilt images and its
upstream repository now describes the project as unmaintained. Pinning the last
legacy image would knowingly introduce a stale security dependency.

Flightcheck only requires a well-defined S3-compatible boundary. Production
operators may use AWS S3 or another supported service.

## Decision

Use Garage v2.3.0 for the reproducible local stack and integration tests. Pin
the image tag, configure a single-node development deployment, and keep object
storage access behind an internal interface.

## Consequences

- Local development retains a maintained open-source S3-compatible service.
- The application cannot rely on vendor-specific MinIO APIs.
- Garage is AGPL-3.0 as an independently deployed service; its code is not
  copied or linked into Flightcheck.
- Production documentation must describe S3 as the supported contract and
  Garage as the local reference implementation.
