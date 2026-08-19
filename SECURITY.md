# Security policy

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability or include patient
data, access tokens, or target credentials in a report. Contact the maintainer
privately through the security advisory feature on GitHub.

Include the affected version, a minimal reproduction using synthetic data, the
expected impact, and any suggested mitigation. Reports are acknowledged within
three business days. Coordinated disclosure is preferred.

## Supported versions

Until the first stable release, only the latest tagged pre-release is
supported. Security fixes are documented in release notes and signed artifacts.

## Product boundary

FHIR Flightcheck evaluates technical controls. It does not certify HIPAA, FDA,
SOC 2, ISO, clinical safety, or legal compliance. Never point active checks at a
system without explicit authorization.
