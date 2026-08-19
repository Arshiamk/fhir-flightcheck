from __future__ import annotations

import asyncio
import os
from typing import TYPE_CHECKING, Protocol, runtime_checkable

if TYPE_CHECKING:
    from .evidence import EvidenceArtifact


@runtime_checkable
class ArtifactStore(Protocol):
    async def upload(self, artifact: "EvidenceArtifact") -> str:
        """Upload artifact bytes. Returns the storage URI (e.g. s3://...)."""
        ...


class NullArtifactStore:
    """Returns the existing urn:sha256: URI unchanged. Used in tests and when S3 is unconfigured."""

    async def upload(self, artifact: "EvidenceArtifact") -> str:
        return str(artifact.metadata.storage_uri)


class S3ArtifactStore:
    """Uploads artifacts to S3 using boto3 via a thread executor."""

    def __init__(
        self,
        *,
        endpoint_url: str | None,
        access_key: str,
        secret_key: str,
        bucket: str,
        region: str,
    ) -> None:
        self._endpoint_url = endpoint_url
        self._access_key = access_key
        self._secret_key = secret_key
        self._bucket = bucket
        self._region = region

    def _put_object(self, key: str, body: bytes, content_type: str) -> None:
        try:
            import boto3  # type: ignore[import-untyped]
        except ImportError as exc:
            raise RuntimeError(
                "boto3 is required for S3 uploads; install it with: pip install boto3"
            ) from exc

        kwargs: dict[str, object] = {
            "aws_access_key_id": self._access_key,
            "aws_secret_access_key": self._secret_key,
            "region_name": self._region,
        }
        if self._endpoint_url:
            kwargs["endpoint_url"] = self._endpoint_url

        client = boto3.client("s3", **kwargs)
        client.put_object(
            Bucket=self._bucket,
            Key=key,
            Body=body,
            ContentType=content_type,
        )

    async def upload(self, artifact: "EvidenceArtifact") -> str:
        run_id = artifact.metadata.run_id
        evidence_id = artifact.metadata.evidence_id
        key = f"evidence/{run_id}/{evidence_id}.json"
        loop = asyncio.get_running_loop()
        await loop.run_in_executor(
            None,
            self._put_object,
            key,
            artifact.content,
            artifact.metadata.media_type,
        )
        return f"s3://{self._bucket}/{key}"


def make_artifact_store() -> ArtifactStore:
    """Create an S3ArtifactStore when env vars are present, otherwise NullArtifactStore."""
    bucket = os.environ.get("S3_BUCKET")
    access_key = os.environ.get("S3_ACCESS_KEY")
    secret_key = os.environ.get("S3_SECRET_KEY")
    if bucket and access_key and secret_key:
        return S3ArtifactStore(
            endpoint_url=os.environ.get("S3_ENDPOINT"),
            access_key=access_key,
            secret_key=secret_key,
            bucket=bucket,
            region=os.environ.get("S3_REGION", "us-east-1"),
        )
    return NullArtifactStore()
