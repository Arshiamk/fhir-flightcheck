from __future__ import annotations

from datetime import UTC, datetime

import httpx
import pytest
import respx

from flightcheck_evaluator.evaluator import HttpProber, UnsafeTargetError
from flightcheck_evaluator.models import RunManifest


def manifest(base_url: str, allow_private: bool = False) -> RunManifest:
    return RunManifest.model_validate(
        {
            "runId": "run:http-001",
            "organizationId": "org:http-001",
            "projectId": "project:http-001",
            "target": {
                "id": "target:http-001",
                "baseUrl": base_url,
                "fhirVersion": "4.0.1",
                "credentialRef": "none",
                "allowPrivateNetwork": allow_private,
            },
            "profile": "test",
            "ruleVersions": {"fhir.discovery.capability": "1.0.0"},
            "createdAt": datetime(2026, 8, 18, tzinfo=UTC).isoformat(),
        }
    )


@respx.mock
async def test_http_prober_collects_fixed_read_only_endpoints() -> None:
    metadata = respx.get("https://synthetic.example/fhir/metadata").mock(
        return_value=httpx.Response(
            200,
            json={"resourceType": "CapabilityStatement", "fhirVersion": "4.0.1"},
            headers={"content-type": "application/fhir+json"},
        )
    )
    smart = respx.get("https://synthetic.example/fhir/.well-known/smart-configuration").mock(
        return_value=httpx.Response(200, json={"capabilities": []})
    )
    search = respx.get("https://synthetic.example/fhir/Patient").mock(
        return_value=httpx.Response(
            200,
            json={"resourceType": "Bundle", "entry": [], "link": []},
        )
    )
    async with httpx.AsyncClient() as client:
        snapshot = await HttpProber(client).collect(manifest("https://synthetic.example/fhir"), {})
    assert snapshot.capability_statement is not None
    assert snapshot.capability_statement.status_code == 200
    assert metadata.called and smart.called and search.called
    request = search.calls.last.request
    assert request.url.params["_count"] == "2"
    assert request.method == "GET"


async def test_http_prober_rejects_unapproved_private_target() -> None:
    async with httpx.AsyncClient() as client:
        with pytest.raises(UnsafeTargetError, match="allowPrivateNetwork"):
            await HttpProber(client).collect(manifest("http://localhost:8080/fhir"), {})
