from __future__ import annotations

import hashlib
from collections.abc import Callable, Iterable
from datetime import UTC, datetime
from typing import Any
from urllib.parse import urlparse

import httpx

from .evidence import EvidenceArtifact, build_evidence
from .models import (
    Capability,
    Finding,
    ProbeResponse,
    ProbeSnapshot,
    RunManifest,
)
from .registry import RuleRegistry

Clock = Callable[[], datetime]


class UnsafeTargetError(ValueError):
    pass


def _validate_target(manifest: RunManifest) -> None:
    parsed = urlparse(str(manifest.target.base_url))
    if parsed.scheme not in {"http", "https"}:
        raise UnsafeTargetError("target URL must use HTTP or HTTPS")
    hostname = parsed.hostname or ""
    local = hostname in {"localhost", "127.0.0.1", "::1"} or hostname.endswith(".local")
    if local and not manifest.target.allow_private_network:
        raise UnsafeTargetError("private targets require allowPrivateNetwork")
    if parsed.scheme != "https" and not (local and manifest.target.allow_private_network):
        raise UnsafeTargetError("remote targets must use HTTPS")


class HttpProber:
    def __init__(self, client: httpx.AsyncClient) -> None:
        self._client = client

    async def _get(self, url: str, *, params: dict[str, str] | None = None) -> ProbeResponse:
        started = datetime.now(UTC)
        try:
            response = await self._client.get(
                url,
                params=params,
                headers={"accept": "application/fhir+json, application/json"},
                follow_redirects=False,
            )
            elapsed = int((datetime.now(UTC) - started).total_seconds() * 1000)
            try:
                body: dict[str, Any] | list[Any] | str | None = response.json()
            except ValueError:
                body = response.text[:4096]
            return ProbeResponse(
                status_code=response.status_code,
                headers={key.lower(): value for key, value in response.headers.items()},
                body=body,
                elapsed_ms=max(0, elapsed),
            )
        except httpx.HTTPError as exc:
            elapsed = int((datetime.now(UTC) - started).total_seconds() * 1000)
            return ProbeResponse(
                status_code=0,
                elapsed_ms=max(0, elapsed),
                error=type(exc).__name__,
            )

    async def collect(self, manifest: RunManifest, fixture: dict[str, Any]) -> ProbeSnapshot:
        _validate_target(manifest)
        base = str(manifest.target.base_url).rstrip("/")
        return ProbeSnapshot(
            capability_statement=await self._get(f"{base}/metadata"),
            smart_configuration=await self._get(f"{base}/.well-known/smart-configuration"),
            search_bundle=await self._get(f"{base}/Patient", params={"_count": "2"}),
            fixture=fixture,
        )


class Evaluator:
    def __init__(self, registry: RuleRegistry, clock: Clock | None = None) -> None:
        self._registry = registry
        self._clock = clock or (lambda: datetime.now(UTC))

    def evaluate(
        self,
        manifest: RunManifest,
        snapshot: ProbeSnapshot,
        granted_capabilities: frozenset[Capability],
        rule_ids: Iterable[str] | None = None,
    ) -> tuple[list[Finding], list[EvidenceArtifact]]:
        selected_ids = list(rule_ids or manifest.rule_versions)
        selected = self._registry.select(selected_ids, granted_capabilities)
        findings: list[Finding] = []
        artifacts: list[EvidenceArtifact] = []
        for rule, evaluator in selected:
            result = evaluator(snapshot)
            observed_at = self._clock()
            artifact = build_evidence(
                run_id=manifest.run_id,
                rule_id=rule.id,
                value=result.evidence,
                created_at=observed_at,
            )
            finding_digest = hashlib.sha256(
                f"{manifest.run_id}:{rule.id}:{rule.version}".encode()
            ).hexdigest()[:24]
            findings.append(
                Finding(
                    finding_id=f"fn:{finding_digest}",
                    run_id=manifest.run_id,
                    rule_id=rule.id,
                    rule_version=rule.version,
                    outcome=result.outcome,
                    severity=rule.severity,
                    title=rule.title,
                    summary=result.summary,
                    evidence_refs=[artifact.metadata.evidence_id],
                    remediation=rule.remediation,
                    observed_at=observed_at,
                )
            )
            artifacts.append(artifact)
        return findings, artifacts
