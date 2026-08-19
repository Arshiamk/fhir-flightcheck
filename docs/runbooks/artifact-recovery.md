# Artifact loss or integrity recovery

Use this runbook for missing objects, digest mismatch, unreadable ciphertext,
lost index, accidental deletion, or report-signature verification failure. A
confirmed mismatch is a security incident until storage corruption and
tampering are distinguished.

1. Make affected artifacts/reports unavailable and label related runs
   `integrity_investigation`; never substitute a successful read from another
   run. Pause retention/deletion and preserve object versions, storage/KMS
   audit, report manifests, database indexes and replica/backup inventory.
2. Bound by organization, bucket/prefix, object digest, key version, region and
   time. Verify tenant and run references, canonical digest, authenticated
   decryption associated data, report signature/trust policy, and final
   coverage. Perform reads through an isolated recovery identity.
3. Choose the newest independently verified source: immutable replica, object
   version, then tested backup. Restore to a quarantine prefix; decrypt and
   verify digest/signature before atomically repairing the authoritative
   reference. Never alter content to make its hash match.
4. If only an index is lost, rebuild it from signed manifests and verified
   objects. If evidence cannot be recovered, mark it lost and the report
   invalid/partial; rerun only with authorization and available pinned inputs.
   Do not claim reproducibility when the original target state is unavailable.
5. Reapply deletion tombstones and retention rules before exposing a restored
   backup. Confirm no cross-tenant objects or expired data were resurrected.
6. Sample adjacent objects, reconcile counts/digests, test offline report
   verification, restore normal reads, and notify affected owners as policy
   requires.

Record recovery point/time actually achieved, irrecoverable artifacts, reports
invalidated or reissued, key availability, root cause and preventive action.
Quarterly exercises must use synthetic data and prove restoration without
granting production-wide object access.
