from __future__ import annotations

import hashlib
import sys
from datetime import UTC, datetime

import pytest

from flightcheck_evaluator.evidence import EvidenceArtifact, build_evidence
from flightcheck_evaluator.storage import (
    NullArtifactStore,
    S3ArtifactStore,
    make_artifact_store,
)


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


class _RecordingS3Client:
    """Captures put_object calls so the upload path can be asserted without network access."""

    def __init__(self) -> None:
        self.calls: list[dict[str, object]] = []

    def put_object(self, **kwargs: object) -> None:
        self.calls.append(kwargs)


class _FakeBoto3:
    def __init__(self, client: _RecordingS3Client) -> None:
        self._client = client
        self.client_kwargs: dict[str, object] = {}

    def client(self, service: str, **kwargs: object) -> _RecordingS3Client:
        assert service == "s3"
        self.client_kwargs = kwargs
        return self._client


def _s3_store(endpoint_url: str | None = None) -> S3ArtifactStore:
    return S3ArtifactStore(
        endpoint_url=endpoint_url,
        access_key="access-key",
        secret_key="secret-key",  # noqa: S106 - test fixture, not a real credential
        bucket="my-bucket",
        region="eu-west-2",
    )


async def test_s3_upload_returns_bucket_scoped_uri_and_sends_object(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    client = _RecordingS3Client()
    monkeypatch.setitem(sys.modules, "boto3", _FakeBoto3(client))
    artifact = _make_artifact()
    store = _s3_store()

    result = await store.upload(artifact)

    expected_key = f"evidence/{artifact.metadata.run_id}/{artifact.metadata.evidence_id}.json"
    assert result == f"s3://my-bucket/{expected_key}"
    assert len(client.calls) == 1
    call = client.calls[0]
    assert call["Bucket"] == "my-bucket"
    assert call["Key"] == expected_key
    assert call["Body"] == artifact.content
    assert call["ContentType"] == artifact.metadata.media_type


async def test_s3_put_object_omits_endpoint_url_when_unset(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeBoto3(_RecordingS3Client())
    monkeypatch.setitem(sys.modules, "boto3", fake)

    await _s3_store().upload(_make_artifact())

    assert "endpoint_url" not in fake.client_kwargs
    assert fake.client_kwargs["region_name"] == "eu-west-2"
    assert fake.client_kwargs["aws_access_key_id"] == "access-key"
    assert fake.client_kwargs["aws_secret_access_key"] == "secret-key"  # noqa: S105


async def test_s3_put_object_forwards_endpoint_url_when_set(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    fake = _FakeBoto3(_RecordingS3Client())
    monkeypatch.setitem(sys.modules, "boto3", fake)

    await _s3_store(endpoint_url="https://minio.internal:9000").upload(_make_artifact())

    assert fake.client_kwargs["endpoint_url"] == "https://minio.internal:9000"


async def test_s3_upload_raises_actionable_error_when_boto3_missing(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    # Setting the entry to None makes `import boto3` raise ImportError.
    monkeypatch.setitem(sys.modules, "boto3", None)

    with pytest.raises(RuntimeError, match="boto3 is required for S3 uploads"):
        await _s3_store().upload(_make_artifact())
