from __future__ import annotations

from typing import Any

from .models import EvaluationResult, Outcome, ProbeResponse, ProbeSnapshot
from .redaction import contains_sensitive_material
from .registry import RuleEvaluator


def _result(outcome: Outcome, summary: str, evidence: dict[str, Any]) -> EvaluationResult:
    return EvaluationResult(outcome=outcome, summary=summary, evidence=evidence)


def _response_json(
    response: ProbeResponse | None, label: str
) -> tuple[dict[str, Any] | None, EvaluationResult | None]:
    if response is None or response.error:
        return None, _result(
            Outcome.INCONCLUSIVE,
            f"{label} was unavailable; no pass was inferred",
            {"probe": label, "error": response.error if response else "not executed"},
        )
    if not isinstance(response.body, dict):
        return None, _result(
            Outcome.FAIL, f"{label} did not return a JSON object", {"probe": label}
        )
    return response.body, None


def capability_statement(snapshot: ProbeSnapshot) -> EvaluationResult:
    body, unavailable = _response_json(snapshot.capability_statement, "CapabilityStatement")
    if unavailable:
        return unavailable
    if body is None:
        return _result(Outcome.PLATFORM_ERROR, "Probe parsing failed", {"probe": "metadata"})
    response = snapshot.capability_statement
    if response is None:
        return _result(Outcome.PLATFORM_ERROR, "Probe response was lost", {"probe": "metadata"})
    valid = (
        response.status_code == 200
        and body.get("resourceType") == "CapabilityStatement"
        and body.get("fhirVersion") == "4.0.1"
    )
    return _result(
        Outcome.PASS if valid else Outcome.FAIL,
        "FHIR R4 CapabilityStatement is consistent"
        if valid
        else "CapabilityStatement is missing, unsuccessful, or not FHIR R4",
        {
            "status": response.status_code,
            "resourceType": body.get("resourceType"),
            "fhirVersion": body.get("fhirVersion"),
        },
    )


def content_type(snapshot: ProbeSnapshot) -> EvaluationResult:
    response = snapshot.capability_statement
    if response is None:
        return _result(
            Outcome.INCONCLUSIVE,
            "CapabilityStatement response was unavailable",
            {"probe": "CapabilityStatement"},
        )
    observed = response.headers.get("content-type", "").lower()
    valid = "application/fhir+json" in observed
    return _result(
        Outcome.PASS if valid else Outcome.FAIL,
        "FHIR JSON content type is explicit" if valid else "FHIR JSON content type is absent",
        {"contentType": observed},
    )


def smart_metadata(snapshot: ProbeSnapshot) -> EvaluationResult:
    body, unavailable = _response_json(snapshot.smart_configuration, "SMART configuration")
    if unavailable:
        return unavailable
    if body is None:
        return _result(Outcome.PLATFORM_ERROR, "Probe parsing failed", {"probe": "SMART"})
    required = {"authorization_endpoint", "token_endpoint", "capabilities"}
    missing = sorted(required - body.keys())
    return _result(
        Outcome.PASS if not missing else Outcome.FAIL,
        "SMART discovery metadata contains required fields"
        if not missing
        else "SMART discovery metadata is incomplete",
        {"missing": missing},
    )


def smart_scopes(snapshot: ProbeSnapshot) -> EvaluationResult:
    body, unavailable = _response_json(snapshot.smart_configuration, "SMART configuration")
    if unavailable:
        return unavailable
    if body is None:
        return _result(Outcome.PLATFORM_ERROR, "Probe parsing failed", {"probe": "SMART"})
    supported = body.get("scopes_supported")
    if not isinstance(supported, list):
        return _result(
            Outcome.INCONCLUSIVE,
            "SMART metadata does not declare supported scopes",
            {"scopesSupported": None},
        )
    unsafe = sorted(scope for scope in supported if isinstance(scope, str) and scope.endswith(".*"))
    return _result(
        Outcome.FAIL if unsafe else Outcome.PASS,
        "SMART scopes include overbroad wildcard access"
        if unsafe
        else "SMART scopes are resource-specific",
        {"overbroadScopes": unsafe},
    )


def pagination_links(snapshot: ProbeSnapshot) -> EvaluationResult:
    body, unavailable = _response_json(snapshot.search_bundle, "FHIR search")
    if unavailable:
        return unavailable
    if body is None:
        return _result(Outcome.PLATFORM_ERROR, "Probe parsing failed", {"probe": "search"})
    links = body.get("link", [])
    relation_urls = {
        item.get("relation"): item.get("url") for item in links if isinstance(item, dict)
    }
    valid = body.get("resourceType") == "Bundle" and bool(relation_urls.get("self"))
    if body.get("total", 0) > len(body.get("entry", [])):
        valid = valid and bool(relation_urls.get("next"))
    return _result(
        Outcome.PASS if valid else Outcome.FAIL,
        "Bundle pagination links are coherent"
        if valid
        else "Bundle pagination links are incomplete",
        {"relations": sorted(key for key in relation_urls if isinstance(key, str))},
    )


def dates_timezones(snapshot: ProbeSnapshot) -> EvaluationResult:
    observations = snapshot.fixture.get("dates", [])
    if not observations:
        return _result(
            Outcome.INCONCLUSIVE,
            "No representative dateTime values were supplied",
            {"dateCount": 0},
        )
    invalid = [
        value
        for value in observations
        if not isinstance(value, str)
        or ("T" in value and not (value.endswith("Z") or "+" in value[10:] or "-" in value[10:]))
    ]
    return _result(
        Outcome.FAIL if invalid else Outcome.PASS,
        "FHIR dateTime values omit timezone offsets"
        if invalid
        else "FHIR dateTime values preserve timezone offsets",
        {"invalidValues": invalid},
    )


def redaction(snapshot: ProbeSnapshot) -> EvaluationResult:
    sample = snapshot.fixture.get("reportedEvidence")
    if sample is None:
        return _result(
            Outcome.INCONCLUSIVE,
            "No reported evidence sample was supplied",
            {"samplePresent": False},
        )
    leaked = contains_sensitive_material(sample)
    return _result(
        Outcome.FAIL if leaked else Outcome.PASS,
        "Evidence contains material requiring redaction"
        if leaked
        else "Evidence sample is redaction-safe",
        {"sensitiveMaterialDetected": leaked},
    )


def _fixture_boolean(path: str, success: str, failure: str) -> RuleEvaluator:
    parts = path.split(".")

    def evaluate(snapshot: ProbeSnapshot) -> EvaluationResult:
        value: Any = snapshot.fixture
        for part in parts:
            if not isinstance(value, dict) or part not in value:
                return _result(
                    Outcome.INCONCLUSIVE,
                    f"Required evidence for {path} was not available",
                    {"evidencePath": path, "available": False},
                )
            value = value[part]
        if not isinstance(value, bool):
            return _result(
                Outcome.INCONCLUSIVE,
                f"Evidence for {path} was not a boolean assertion",
                {"evidencePath": path, "observedType": type(value).__name__},
            )
        return _result(
            Outcome.PASS if value else Outcome.FAIL,
            success if value else failure,
            {"evidencePath": path, "observed": value},
        )

    return evaluate


def evaluator_map() -> dict[str, RuleEvaluator]:
    checks: dict[str, RuleEvaluator] = {
        "capability_statement": capability_statement,
        "content_type": content_type,
        "smart_metadata": smart_metadata,
        "smart_scopes": smart_scopes,
        "pagination_links": pagination_links,
        "dates_timezones": dates_timezones,
        "redaction": redaction,
    }
    boolean_checks: dict[str, tuple[str, str, str]] = {
        "references": (
            "fhir.referencesValid",
            "FHIR references resolve",
            "FHIR references are broken",
        ),
        "terminology": (
            "fhir.terminologyValid",
            "Terminology bindings are valid",
            "Terminology bindings are invalid",
        ),
        "narrative": (
            "fhir.narrativeSafe",
            "Narrative is safely generated",
            "Narrative contains unsafe markup",
        ),
        "modifier_extensions": (
            "fhir.modifierExtensionsHandled",
            "Modifier extensions are explicitly handled",
            "Modifier extensions are silently ignored",
        ),
        "unknown_extensions": (
            "fhir.unknownExtensionsPreserved",
            "Unknown non-modifier extensions are preserved",
            "Unknown non-modifier extensions are discarded",
        ),
        "retry": (
            "reliability.retryIdempotent",
            "Retries are idempotent",
            "Retries duplicate effects",
        ),
        "retry_after": (
            "reliability.retryAfterHonored",
            "Retry-After is honored",
            "Retry-After is ignored",
        ),
        "timeouts": (
            "reliability.timeoutsBounded",
            "Timeouts are bounded",
            "Timeouts exceed the budget",
        ),
        "page_loops": (
            "reliability.paginationLoopDetected",
            "Pagination loops are detected",
            "Pagination loops are not detected",
        ),
        "duplicates": (
            "reliability.duplicatePagesDetected",
            "Duplicate pages are detected",
            "Duplicate pages go undetected",
        ),
        "concurrency": (
            "reliability.concurrencyBounded",
            "Concurrency is bounded",
            "Concurrency is unbounded",
        ),
        "version_conflicts": (
            "reliability.versionConflictsHandled",
            "Version conflicts are handled without overwrite",
            "Version conflicts can overwrite newer state",
        ),
        "security_headers": (
            "security.headersPresent",
            "Security headers are present",
            "Security headers are missing",
        ),
        "token_claims": (
            "security.tokenClaimsValid",
            "Token claims are constrained",
            "Token claims are invalid",
        ),
        "audit": (
            "security.auditComplete",
            "Audit evidence is correlated",
            "Audit evidence is incomplete",
        ),
        "redirects": (
            "security.redirectsSafe",
            "Redirect allowlist is enforced",
            "Unsafe redirects are accepted",
        ),
        "patient_isolation": (
            "security.patientIsolationEnforced",
            "Patient context isolation is enforced",
            "Patient context can cross an authorization boundary",
        ),
        "retention_export": (
            "security.retentionExportDeclared",
            "Retention and export behavior is declared",
            "Retention or export behavior is undeclared",
        ),
        "grounding": (
            "aiSafety.grounded",
            "Output is grounded in supplied facts",
            "Output contains unsupported claims",
        ),
        "provenance": (
            "aiSafety.provenancePresent",
            "Resource provenance is present",
            "Resource provenance is absent",
        ),
        "prompt_injection": (
            "aiSafety.promptInjectionResisted",
            "Prompt injection is rejected",
            "Prompt injection influenced output",
        ),
        "unsafe_writes": (
            "aiSafety.unsafeWritesBlocked",
            "Autonomous clinical writes are blocked",
            "An unsafe autonomous write was attempted",
        ),
        "least_privilege": (
            "aiSafety.leastPrivilegeTools",
            "Tool use is least privilege",
            "Tool use exceeds granted capability",
        ),
        "human_review": (
            "aiSafety.humanReviewRouted",
            "High-risk output routes to human review",
            "High-risk output bypasses human review",
        ),
        "model_trace": (
            "aiSafety.modelTraceable",
            "Model version is traceable",
            "Model version is not traceable",
        ),
        "citations": (
            "aiSafety.citationsValid",
            "Citations resolve to supporting resources",
            "Citations are missing or do not support the claims",
        ),
        "abstention": (
            "aiSafety.abstainsWhenUnsupported",
            "Unsupported requests trigger safe abstention",
            "Unsupported requests produce ungrounded clinical output",
        ),
    }
    for name, (path, success, failure) in boolean_checks.items():
        checks[name] = _fixture_boolean(path, success, failure)
    return checks
