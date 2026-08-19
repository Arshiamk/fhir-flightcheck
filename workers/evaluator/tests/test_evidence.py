from __future__ import annotations

import hashlib
from datetime import UTC, datetime

from hypothesis import given
from hypothesis import strategies as st

from flightcheck_evaluator.evidence import build_evidence, canonical_json
from flightcheck_evaluator.redaction import redact

NOW = datetime(2026, 8, 18, tzinfo=UTC)
JSON_SCALARS = st.none() | st.booleans() | st.integers() | st.text(max_size=50)
JSON_VALUES = st.recursive(
    JSON_SCALARS,
    lambda children: (
        st.lists(children, max_size=5)
        | st.dictionaries(st.text(min_size=1, max_size=12), children, max_size=5)
    ),
    max_leaves=20,
)


@given(JSON_VALUES)
def test_canonical_hash_is_deterministic(value: object) -> None:
    first = build_evidence(run_id="run:test", rule_id="test.rule.hash", value=value, created_at=NOW)
    second = build_evidence(
        run_id="run:test", rule_id="test.rule.hash", value=value, created_at=NOW
    )
    assert first.content == second.content
    assert first.metadata.sha256 == second.metadata.sha256
    assert first.metadata.sha256 == hashlib.sha256(first.content).hexdigest()


def test_redaction_precedes_hash_and_storage() -> None:
    artifact = build_evidence(
        run_id="run:test",
        rule_id="security.evidence.redaction",
        value={
            "authorization": "Bearer abcdefghijklmnopqrstuvwxyz",
            "nested": {"birthDate": "1970-01-01", "safe": "kept"},
        },
        created_at=NOW,
    )
    rendered = artifact.content.decode()
    assert "abcdefghijklmnopqrstuvwxyz" not in rendered
    assert "1970-01-01" not in rendered
    assert '"safe":"kept"' in rendered
    assert artifact.metadata.redaction_status == "redacted"


@given(st.dictionaries(st.text(min_size=1), JSON_SCALARS, max_size=8))
def test_redaction_is_idempotent(value: dict[str, object]) -> None:
    first = redact(value)
    second = redact(first.value)
    assert canonical_json(first.value) == canonical_json(second.value)


def test_evidence_id_binds_to_run_id() -> None:
    """Cross-tenant evidence swap: same payload, different run_ids → same content hash
    but distinct run_id in metadata. The control-plane validator uses run_id to reject
    evidence belonging to the wrong run even when the content hash matches."""
    payload = {"observation": "glucose", "value": 5.4}

    artifact_a = build_evidence(
        run_id="run:tenant-a", rule_id="test.rule.swap", value=payload, created_at=NOW
    )
    artifact_b = build_evidence(
        run_id="run:tenant-b", rule_id="test.rule.swap", value=payload, created_at=NOW
    )

    # Content-addressed IDs are identical — same data produces the same hash.
    assert artifact_a.metadata.evidence_id == artifact_b.metadata.evidence_id
    assert artifact_a.metadata.sha256 == artifact_b.metadata.sha256
    assert artifact_a.content == artifact_b.content

    # The run_id binding differs, so the control-plane can reject the wrong one.
    assert artifact_a.metadata.run_id != artifact_b.metadata.run_id
    assert artifact_a.metadata.run_id == "run:tenant-a"
    assert artifact_b.metadata.run_id == "run:tenant-b"


def test_phi_fields_are_redacted_by_default() -> None:
    """PHI redaction canary: dates, names, identifiers and telecoms must be stripped;
    a non-sensitive field must survive."""
    phi_value = {
        "birthDate": "1985-03-14",
        "name": [{"family": "Smith", "given": ["John"]}],
        "identifier": [{"value": "NHS-123456789"}],
        "telecom": [{"value": "+447700900000"}],
        "safe_field": "keep_this",
    }

    artifact = build_evidence(
        run_id="run:phi-test",
        rule_id="security.evidence.redaction",
        value=phi_value,
        created_at=NOW,
    )
    rendered = artifact.content.decode()

    for phi in ("1985-03-14", "Smith", "John", "NHS-123456789", "+447700900000"):
        assert phi not in rendered, f"PHI value {phi!r} was not redacted"

    assert "keep_this" in rendered
    assert artifact.metadata.redaction_status == "redacted"


def test_canonical_json_rejects_mutated_object() -> None:
    """Object mutation canary: mutating the source dict after build_evidence has been
    called must not alter the already-captured bytes, and rebuilding from the original
    dict must produce the same SHA-256 (determinism + immutability)."""
    source: dict[str, object] = {"result": "stable", "count": 42}

    original = build_evidence(
        run_id="run:mutation", rule_id="test.rule.canary", value=source, created_at=NOW
    )
    captured_content = original.content
    captured_sha256 = original.metadata.sha256

    # Mutate the source dict after the artifact was built.
    source["result"] = "MUTATED"
    source["injected"] = "extra-field"

    # The already-built artifact is immutable — mutations are not retroactive.
    assert original.content == captured_content
    assert original.metadata.sha256 == captured_sha256

    # Rebuild from the *original* (unmodified) values to confirm determinism.
    rebuilt = build_evidence(
        run_id="run:mutation",
        rule_id="test.rule.canary",
        value={"result": "stable", "count": 42},
        created_at=NOW,
    )
    assert rebuilt.content == captured_content
    assert rebuilt.metadata.sha256 == captured_sha256
