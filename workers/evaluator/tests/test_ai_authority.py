from __future__ import annotations

from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import pytest

from flightcheck_evaluator.evaluator import Evaluator
from flightcheck_evaluator.models import Capability, Outcome, ProbeSnapshot, RunManifest
from flightcheck_evaluator.registry import RegistryError, RuleRegistry
from flightcheck_evaluator.rules import evaluator_map

ROOT = Path(__file__).resolve().parents[3]
PACKS = ROOT / "packages" / "rule-packs"
NOW = datetime(2026, 8, 18, 12, 0, tzinfo=UTC)


def _registry() -> RuleRegistry:
    result = RuleRegistry(evaluator_map())
    result.load_directory(PACKS)
    return result


def _manifest(rule_versions: dict[str, str]) -> RunManifest:
    return RunManifest.model_validate(
        {
            "schemaVersion": "1.0.0",
            "runId": "run:ai-authority-001",
            "organizationId": "org:ai-authority-001",
            "projectId": "project:ai-authority-001",
            "target": {
                "id": "target:ai-authority-001",
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


def _snapshot(fixture: dict[str, Any]) -> ProbeSnapshot:
    return ProbeSnapshot(fixture=fixture)


def _evaluate_single(rule_id: str, fixture: dict[str, Any]) -> Outcome:
    reg = _registry()
    rule = next(r for r in reg.rules if r.id == rule_id)
    findings, _ = Evaluator(reg, clock=lambda: NOW).evaluate(
        _manifest({rule_id: rule.version}),
        _snapshot(fixture),
        frozenset({Capability.FIXTURES}),
    )
    assert len(findings) == 1
    return findings[0].outcome


def test_ai_write_prohibition_blocks_unapproved_write() -> None:
    """ai.write.prohibition must fail when unsafeWritesBlocked is False."""
    outcome = _evaluate_single(
        "ai.write.prohibition",
        {"aiSafety": {"unsafeWritesBlocked": False}},
    )
    assert outcome == Outcome.FAIL


def test_ai_write_prohibition_passes_when_write_blocked() -> None:
    """ai.write.prohibition must pass when unsafeWritesBlocked is True."""
    outcome = _evaluate_single(
        "ai.write.prohibition",
        {"aiSafety": {"unsafeWritesBlocked": True}},
    )
    assert outcome == Outcome.PASS


def test_ai_tool_model_allowlist_cannot_be_bypassed_with_empty_fixture() -> None:
    """Absence of evidence must not be treated as approval.
    An empty ProbeSnapshot must yield INCONCLUSIVE, not PASS."""
    outcome = _evaluate_single("ai.write.prohibition", {})
    assert outcome == Outcome.INCONCLUSIVE, (
        f"Empty fixture must not produce PASS; got {outcome!r}"
    )


def test_ai_human_review_requirement_fails_on_direct_write() -> None:
    """ai.review.human must fail when humanReviewRouted is False."""
    outcome = _evaluate_single(
        "ai.review.human",
        {"aiSafety": {"humanReviewRouted": False}},
    )
    assert outcome == Outcome.FAIL


def test_ai_prompt_injection_resistance_fails_on_malicious_input() -> None:
    """ai.prompt.injection must fail when promptInjectionResisted is False."""
    outcome = _evaluate_single(
        "ai.prompt.injection",
        {"aiSafety": {"promptInjectionResisted": False}},
    )
    assert outcome == Outcome.FAIL


def test_ai_rules_require_fixtures_capability() -> None:
    """Evaluating AI rules without the FIXTURES capability must raise RegistryError.
    The capability gate must not be bypassed even when the fixture is empty."""
    reg = _registry()
    ai_rule_ids = ["ai.write.prohibition", "ai.review.human", "ai.prompt.injection"]
    versions = {rid: next(r.version for r in reg.rules if r.id == rid) for rid in ai_rule_ids}

    with pytest.raises(RegistryError, match="ungranted"):
        Evaluator(reg).evaluate(
            _manifest(versions),
            ProbeSnapshot(),
            frozenset(),  # no capabilities granted
        )
