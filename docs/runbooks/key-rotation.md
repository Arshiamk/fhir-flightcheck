# Key rotation and suspected key compromise

Identify key purpose (credential KEK, evidence KEK, report signer, OIDC/client,
queue, object-store, or target credential), provider key/version, environments,
and data/services in scope. A routine rotation can be scheduled; suspected
exposure is a security incident.

1. For suspected compromise, restrict new sensitive runs/exports/signing,
   preserve KMS and access audit, revoke active sessions/grants where safe, and
   notify the incident commander. Never copy key material into a ticket.
2. Create a new purpose-specific key version with the same or stricter policy.
   Grant only named workload identities; keep old decrypt/verify ability
   temporarily. Deploy writers/signers to the new version and verify key IDs in
   new ciphertext/signatures.
3. Inventory affected wrapped DEKs. Rewrap in bounded, resumable batches using
   KMS-native re-encryption where possible; verify counts, associated data, and
   random decrypt samples. Do not rewrite evidence content or change its digest.
4. For a signing key, publish the new public key/trust metadata. Keep the old
   public key for historical verification. If compromised, mark its validity
   interval/revocation status and reissue reports only from independently
   verified evidence; do not imply old signatures remain trustworthy.
5. Rotate credentials or tokens that the exposed key protected. Rewrapping
   does not undo plaintext exposure. Complete backup and disaster-recovery key
   handling before disabling the old key.
6. Disable old encrypt/sign use, observe for failures, then schedule destruction
   after retention, rollback and legal-hold requirements. Two authorized people
   approve destructive key deletion.

Exit when new writes use the new version, inventory and sampling reconcile,
services and offline verification pass, old key use is explained, dependent
credentials are handled, and an audited timeline/impact assessment exists.
Rollback means restoring old **use** while it remains trusted; never roll back
to a key believed compromised.
