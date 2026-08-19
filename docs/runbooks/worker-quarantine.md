# Worker quarantine

**Triggers:** sandbox escape indicator, unexpected destination, credential or
tenant mismatch, tampered rule, malware alert, unexplained resource use, or
repeated invalid evidence.

1. Treat suspected escape, credential access, or cross-tenant behavior as a
   security incident. Stop routing new work to the worker pool. Revoke its
   workload/queue/object-store grants and destination credentials. Network
   isolate affected instances; do not let them finish work.
2. Mark their active attempts non-authoritative. The control plane rejects late
   completions. Pause the exact rule-pack digest and dependent active probes
   globally or in the narrowest safe scope.
3. Preserve provider snapshots where policy allows, immutable image/rule
   digests, process and network telemetry, queue metadata, grants, audit events
   and run IDs. Do not execute suspect artifacts on an analyst workstation or
   capture FHIR payloads unnecessarily.
4. Determine affected build, image, rule digest, worker identities, tenants,
   destinations, credentials and time range. Check object access and result
   submissions for the same identities. Rotate exposed grants and upstream
   target credentials.
5. Rebuild from reviewed source and trusted pinned dependencies in a clean
   environment; never return an isolated instance to service. Verify signature,
   SBOM/scans, sandbox negative tests, egress denial and canary execution before
   gradual replacement.
6. Re-run affected rules only with tenant authorization and immutable original
   inputs. Use new attempt IDs and preserve invalidated results for audit.

Exit requires clean replacement capacity, revoked suspect identities, bounded
impact, validated evidence/results, and monitoring with no recurrence. If
tenant boundary or restricted data may have been crossed, follow the
cross-tenant runbook and notification process.
