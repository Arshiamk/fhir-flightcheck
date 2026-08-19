from __future__ import annotations

import json
from collections import Counter
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, cast

import pytest
from jsonschema import Draft202012Validator  # type: ignore[import-untyped]

from flightcheck_evaluator.evaluator import Evaluator
from flightcheck_evaluator.models import (
    Capability,
    Outcome,
    ProbeResponse,
    ProbeSnapshot,
    RunManifest,
)
from flightcheck_evaluator.registry import RegistryError, RuleRegistry
from flightcheck_evaluator.rules import evaluator_map

ROOT = Path(__file__).resolve().parents[3]
PACKS = ROOT / "packages" / "rule-packs"
FIXTURES = ROOT / "fixtures" / "synthea"
SCHEMA = ROOT / "packages" / "contracts" / "schema" / "flightcheck.schema.json"
NOW = datetime(2026, 8, 18, 12, 0, tzinfo=UTC)


def registry() -> RuleRegistry:
    result = RuleRegistry(evaluator_map())
    result.load_directory(PACKS)
    return result


def fixture(name: str) -> dict[str, Any]:
    return cast(dict[str, Any], json.loads((FIXTURES / name).read_text(encoding="utf-8")))


def snapshot(payload: dict[str, Any]) -> ProbeSnapshot:
    probes = payload["probes"]
    return ProbeSnapshot(
        capability_statement=ProbeResponse.model_validate(probes["capabilityStatement"]),
        smart_configuration=ProbeResponse.model_validate(probes["smartConfiguration"]),
        search_bundle=ProbeResponse.model_validate(probes["searchBundle"]),
        fixture=payload,
    )


def manifest(rule_versions: dict[str, str]) -> RunManifest:
    return RunManifest.model_validate(
        {
            "schemaVersion": "1.0.0",
            "runId": "run:test-001",
            "organizationId": "org:test-001",
            "projectId": "project:test-001",
            "target": {
                "id": "target:test-001",
                "baseUrl": "https://synthetic.example/fhir",
                "fhirVersion": "4.0.1",
                "credentialRef": "secret:test",
            },
            "profile": "startup-r4",
            "ruleVersions": rule_versions,
            "fixtureVersion": "synthea-v1",
            "createdAt": NOW.isoformat(),
        }
    )


def test_catalog_has_35_contract_valid_deterministic_rules() -> None:
    loaded = registry().rules
    assert len(loaded) == 35
    assert Counter(rule.category.value for rule in loaded) == {
        "fhir": 9,
        "reliability": 8,
        "security": 9,
        "ai-safety": 9,
    }
    assert all(rule.deterministic for rule in loaded)
    schema = json.loads(SCHEMA.read_text(encoding="utf-8"))
    validator = Draft202012Validator(schema)
    for rule in loaded:
        validator.validate(rule.model_dump(mode="json", by_alias=True, exclude_none=True))


def test_healthy_fixture_passes_all_rules() -> None:
    rules = registry()
    versions = {rule.id: rule.version for rule in rules.rules}
    findings, evidence = Evaluator(rules, clock=lambda: NOW).evaluate(
        manifest(versions),
        snapshot(fixture("healthy.json")),
        frozenset({Capability.NETWORK, Capability.FIXTURES}),
    )
    assert len(findings) == 35
    assert len(evidence) == 35
    assert {finding.outcome for finding in findings} == {Outcome.PASS}
    assert len({item.metadata.evidence_id for item in evidence}) >= 20


@pytest.mark.parametrize(
    ("name", "failed_rules"),
    [
        (
            "broken-fhir.json",
            {
                "fhir.reference.integrity",
                "fhir.terminology.binding",
                "fhir.datetime.timezone",
                "fhir.extension.modifier",
                "fhir.extension.unknown",
            },
        ),
        (
            "broken-reliability.json",
            {
                "fhir.search.pagination",
                "reliability.retry.idempotency",
                "reliability.throttle.retryafter",
                "reliability.timeout.budget",
                "reliability.pagination.loop",
                "reliability.pagination.duplicates",
                "reliability.concurrency.bound",
                "reliability.recovery.evidence",
                "reliability.version.conflict",
            },
        ),
        (
            "broken-security-ai.json",
            {
                "security.smart.scopes",
                "security.evidence.redaction",
                "security.token.claims",
                "security.patient.isolation",
                "security.audit.correlation",
                "ai.prompt.injection",
                "ai.write.prohibition",
                "ai.provenance.resources",
                "ai.review.human",
                "ai.citation.integrity",
                "ai.abstention.unsupported",
            },
        ),
    ],
)
def test_broken_scenarios_fail_for_intended_controls(name: str, failed_rules: set[str]) -> None:
    rules = registry()
    versions = {rule.id: rule.version for rule in rules.rules}
    findings, _ = Evaluator(rules, clock=lambda: NOW).evaluate(
        manifest(versions),
        snapshot(fixture(name)),
        frozenset({Capability.NETWORK, Capability.FIXTURES}),
    )
    observed = {item.rule_id for item in findings if item.outcome == Outcome.FAIL}
    assert failed_rules <= observed


def test_missing_behavior_evidence_is_inconclusive_not_pass() -> None:
    rules = registry()
    selected = {"reliability.retry.idempotency": "1.0.0"}
    findings, _ = Evaluator(rules, clock=lambda: NOW).evaluate(
        manifest(selected),
        ProbeSnapshot(),
        frozenset({Capability.FIXTURES}),
    )
    assert findings[0].outcome == Outcome.INCONCLUSIVE
    assert "not available" in findings[0].summary


def test_registry_rejects_ungranted_capability() -> None:
    rules = registry()
    selected = {"fhir.discovery.capability": "1.0.0"}
    with pytest.raises(RegistryError, match="ungranted"):
        Evaluator(rules).evaluate(manifest(selected), ProbeSnapshot(), frozenset())
