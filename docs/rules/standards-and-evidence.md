# Standards and evidence policy

Flightcheck rules turn narrowly defined technical expectations into
reproducible observations. A passing rule means only that its stated condition
was demonstrated for the pinned target, inputs, and rule version.

## First release baseline

- HL7 FHIR R4 4.0.1
- SMART App Launch metadata and authorization behavior
- Pinned implementation-guide packages for profile-specific checks
- FHIR implementer safety checklist items where they can be observed safely
- RFC 9457 problem details for Flightcheck API errors
- SARIF 2.1.0 for CI findings

Implementation-guide and terminology packages are resolved during a reviewed
release update, locked by exact package version and digest, and recorded in each
run manifest. Flightcheck never silently changes a target's validation profile.

## Evidence requirements

Every conclusive result records:

1. the stable rule identifier and semantic version;
2. the target and input snapshot from the run manifest;
3. the observation time and logical execution identifier;
4. redacted evidence or a content-addressed evidence reference;
5. the standard or product expectation being tested;
6. the severity and decision effect; and
7. concrete remediation.

When a check cannot obtain required evidence, it returns `inconclusive`.
Infrastructure defects return `platform_error`. Neither outcome is a pass.

## Validator integration

Flightcheck orchestrates trusted FHIR validators for profile and terminology
validation. It does not present a partial home-grown validator as equivalent.
Validator binaries, packages, configuration, and output are pinned as evidence.

## AI evaluation

Deterministic assertions and golden synthetic cases are primary. A model judge
is optional, versioned, and supplementary. No blocking result rests solely on a
model's subjective rating.

## Regulatory language

Rules may reference controls that are relevant to regulated environments, but
reports do not claim HIPAA, FDA, SOC 2, ISO, or clinical-safety certification.
Legal and clinical assurance require organization-specific evidence and
qualified review outside this tool.
