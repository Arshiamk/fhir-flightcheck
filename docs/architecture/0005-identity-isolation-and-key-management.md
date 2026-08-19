# ADR 0005: Identity, tenant isolation, and key management

**Status:** Proposed  
**Date:** 2026-08-18

## Context

Flightcheck handles target credentials and potentially sensitive evidence in a
multi-tenant asynchronous system. Application checks alone are insufficient:
browser, API, database, queue, worker, object-store, and key-service boundaries
must agree on tenant and authority. This ADR is a design decision, not a claim
that the current prototype implements these controls.

## Decision

### Identity and authorization

Human users authenticate through OIDC Authorization Code flow with PKCE.
Validate issuer, signature, audience, expiry, nonce, and authorized redirect
URI. Sessions use opaque, rotated, `Secure`, `HttpOnly`, `SameSite=Lax` cookies;
the browser does not store target credentials.

Services use platform workload identity, preferably OIDC/SPIFFE or a cloud
equivalent, to obtain short-lived service credentials. Static bootstrap
credentials are a migration exception and are never shared between services.
The control plane accepts no tenant identity asserted only in request JSON.

Coarse roles are `organization_admin`, `project_admin`, `operator`, `analyst`,
and `viewer`. Permissions are action/resource pairs; project membership and
policy add scope beneath the role. Credential administration, active-probe
approval, key administration, baseline changes, evidence export, and audit
access are separate permissions. Service identities have separate roles and
cannot impersonate people. Denials and sensitive grants are audited.

### Database isolation

Every tenant-owned row carries non-null `organization_id`; project descendants
also use tenant-qualified foreign keys. API transactions set a verified
organization ID using transaction-local database context. Row-level security
policies apply `USING` and `WITH CHECK` to reads and writes and are forced on
table owners where PostgreSQL permits it.

The runtime database role is not a superuser, table owner, or holder of
`BYPASSRLS`. Connection checkout starts a transaction and sets context; check-in
rolls it back so pooled state cannot leak. Background jobs derive context from a
validated, scoped grant, not a queue field alone. Migrations and break-glass
maintenance use distinct identities, time-limited access, and audit.

Object keys are prefixed by organization and opaque run/evidence IDs. The
control plane issues short-lived access constrained to an exact object or
prefix; callers cannot choose an arbitrary bucket key. Queue subjects and
consumer permissions are service-scoped; tenant authorization remains enforced
at the control plane because a subject name is not an authorization boundary.

### Envelope encryption

Each secret or sensitive retained artifact is encrypted with a fresh random
256-bit data-encryption key (DEK) using an authenticated mode such as AES-256-GCM
or ChaCha20-Poly1305. Associated data binds at least schema version,
organization ID, object type, object ID, and key version. The plaintext DEK is
wrapped by a non-exportable key-encryption key (KEK) in KMS/HSM and then erased
from process memory on a best-effort basis. Store ciphertext, nonce, algorithm,
wrapped DEK, KEK identifier/version, and associated-data version; never store
the plaintext DEK.

KEKs are environment- and purpose-specific: target credentials, retained
evidence, and report signing do not share keys. Report signing uses an
asymmetric non-exportable signing key, not an encryption KEK. Workers do not
receive KEKs: an authorized broker unwraps only for a short-lived grant bound to
tenant, run, target, destination, scope, and expiry.

Rotation creates a new KEK version for writes, rewraps DEKs without decrypting
bulk ciphertext where the provider supports it, verifies a sample and inventory
counts, and only then disables the old version. Destruction follows retention
and recovery windows. Compromise rotation follows the key-rotation runbook and
also revokes dependent credentials; rewrapping alone does not remove plaintext
already exposed to an attacker.

## Consequences

- Authorization must be tested at API, database, worker grant, and object-store
  boundaries.
- Local development may use a documented development key provider, but its keys
  and security properties are not equivalent to managed KMS/HSM.
- RLS is defense in depth, not a substitute for application authorization.
- Encryption protects stored media; it does not prevent authorized services
  from mishandling plaintext, so redaction and least privilege remain required.
