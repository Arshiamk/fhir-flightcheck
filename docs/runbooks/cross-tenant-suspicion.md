# Suspected cross-tenant access

Any credible indication that one organization could read, mutate, execute
against, export, or infer another organization's data is a highest-priority
security incident. Do not test the suspicion by opening another tenant's FHIR
payload or by placing details in a public issue.

1. Start the restricted incident channel and legal/privacy notification
   process. Record UTC time and tenant-safe opaque IDs. Limit support access.
2. Contain the suspected path: disable the affected endpoint/export/rule or
   worker grant; revoke implicated sessions and service credentials; stop new
   work in affected pools. Prefer narrow isolation, but fail closed if scope is
   unknown. Preserve database, KMS, object-store, queue, identity and application
   audit before routine retention expires.
3. Build a read-only timeline from authorization decisions, transaction tenant
   context, database role/RLS changes, object keys and grants, queue envelopes,
   worker identity, exports and deploy digests. Search by opaque identifiers,
   not patient attributes. Treat absence of application logs as inconclusive.
4. Determine whether access was merely attempted, authorized incorrectly, or
   completed; which actions and fields were exposed/changed; affected tenants,
   users, services, targets and time; and whether evidence/reports need
   invalidation. Avoid collecting additional restricted content.
5. Fix the boundary and add a regression test reproducing the exact path.
   Validate API authorization, forced RLS under the runtime role, pooled
   transaction reset, tenant-qualified foreign keys, worker grants and object
   prefixes. Rebuild compromised workers and rotate exposed credentials/keys.
6. Before recovery, run the cross-tenant negative suite in isolation and review
   privileged-role changes. Re-enable gradually with denial and object-access
   monitoring. Repair records only from verified audit/evidence and preserve
   originals.

Exit requires containment, reviewed scope and timeline, required notifications,
validated tenant boundaries, rotated credentials, affected report disposition,
and corrective owners. Claims such as “no PHI accessed” require affirmative
provider/database evidence; do not infer them from a missing application log.
