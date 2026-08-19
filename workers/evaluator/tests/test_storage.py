from __future__ import annotations

import hashlib
from datetime import UTC, datetime

import pytest

from flightcheck_evaluator.evidence import EvidenceArtifact, build_evidence
from flightcheck_evaluator.storage import NullArtifactStore, make_artifact_store


def _make_artifact(run_id: str = "run:test-001") -> EvidenceArtifact:
    return build_evidence(
        run_id=run_id,
        rule_id="fhir.test.rule",
        value={"key": "value"},
        created_at=datetime(2026, 8, 19, tzinfo=UTC),
    )


async def test_null_store_returns_urn_sha256_uri() -> None:
    artifact = _make_artifact()
    store = NullArtifactStore()
    result = await store.upload(artifact)
    assert result.startswith("urn:sha256:")
    expected_digest = artifact.metadata.sha256
    assert result == f"urn:sha256:{expected_digest}"


async def test_null_store_uri_matches_original_storage_uri() -> None:
    artifact = _make_artifact()
    store = NullArtifactStore()
    result = await store.upload(artifact)
    assert result == str(artifact.metadata.storage_uri)


async def test_make_artifact_store_returns_null_when_env_missing(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delenv("S3_BUCKET", raising=False)
    monkeypatch.delenv("S3_ACCESS_KEY", raising=False)
    monkeypatch.delenv("S3_SECRET_KEY", raising=False)
    store = make_artifact_store()
    assert isinstance(store, NullArtifactStore)


async def test_make_artifact_store_returns_s3_when_env_present(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("S3_BUCKET", "my-bucket")
    monkeypatch.setenv("S3_ACCESS_KEY", "access-key")
    monkeypatch.setenv("S3_SECRET_KEY", "secret-key")
    from flightcheck_evaluator.storage import S3ArtifactStore

    store = make_artifact_store()
    assert isinstance(store, S3ArtifactStore)


async def test_make_artifact_store_returns_null_when_only_partial_env(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("S3_BUCKET", "my-bucket")
    monkeypatch.delenv("S3_ACCESS_KEY", raising=False)
    monkeypatch.delenv("S3_SECRET_KEY", raising=False)
    store = make_artifact_store()
    assert isinstance(store, NullArtifactStore)


async def test_null_store_upload_preserves_content_integrity() -> None:
    artifact = _make_artifact()
    content_hash = hashlib.sha256(artifact.content).hexdigest()
    store = NullArtifactStore()
    await store.upload(artifact)
    assert hashlib.sha256(artifact.content).hexdigest() == content_hash
