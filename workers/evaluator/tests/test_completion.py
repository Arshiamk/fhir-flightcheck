from __future__ import annotations

from datetime import UTC, datetime

import httpx
import pytest
import respx

from flightcheck_evaluator.worker import (
    CompletionClient,
    CompletionPayload,
    PermanentJobError,
    TransientJobError,
    WorkerConfig,
    WorkerConfigurationError,
)

TOKEN = "worker-secret-token-that-must-never-leak"  # noqa: S105
NOW = datetime(2026, 8, 18, tzinfo=UTC)


def config() -> WorkerConfig:
    return WorkerConfig.model_validate(
        {
            "controlPlaneUrl": "https://control.example",
            "workerToken": TOKEN,
        }
    )


def payload() -> CompletionPayload:
    return CompletionPayload.model_validate(
        {
            "jobId": "job:test-001",
            "runId": "run:test-001",
            "findings": [
                {
                    "findingId": "fn:test-001",
                    "runId": "run:test-001",
                    "ruleId": "fhir.narrative.safety",
                    "ruleVersion": "1.0.0",
                    "outcome": "pass",
                    "severity": "medium",
                    "title": "Narrative markup safety",
                    "summary": "Narrative is safely generated",
                    "evidenceRefs": ["ev:test-001"],
                    "remediation": "Sanitize generated XHTML before rendering.",
                    "observedAt": NOW.isoformat(),
                }
            ],
            "evidence": [
                {
                    "evidenceId": "ev:test-001",
                    "runId": "run:test-001",
                    "mediaType": "application/json",
                    "sha256": "a" * 64,
                    "sizeBytes": 2,
                    "storageUri": f"urn:sha256:{'a' * 64}",
                    "redactionStatus": "not-required",
                    "sourceRuleId": "fhir.narrative.safety",
                    "createdAt": NOW.isoformat(),
                }
            ],
        }
    )


def report(run_id: str = "run:test-001") -> dict[str, str]:
    return {
        "schemaVersion": "1.0.0",
        "reportId": "report:test-001",
        "runId": run_id,
        "decision": "ready",
    }


@pytest.mark.parametrize("status", [200, 201])
@respx.mock
async def test_completion_accepts_success_and_duplicate(status: int) -> None:
    route = respx.post("https://control.example/internal/v1/jobs/job%3Atest-001/complete").mock(
        return_value=httpx.Response(status, json=report())
    )
    async with httpx.AsyncClient() as client:
        await CompletionClient(client, config()).submit(payload())
    request = route.calls.last.request
    assert request.headers["authorization"] == f"Bearer {TOKEN}"
    body = request.content.decode()
    assert '"jobId":"job:test-001"' in body
    assert '"runId":"run:test-001"' in body
    assert '"findings":' in body and '"evidence":' in body


@pytest.mark.parametrize("status", [408, 429, 500, 503])
@respx.mock
async def test_completion_classifies_transient_statuses(status: int) -> None:
    respx.post("https://control.example/internal/v1/jobs/job%3Atest-001/complete").mock(
        return_value=httpx.Response(status, text=TOKEN)
    )
    async with httpx.AsyncClient() as client:
        with pytest.raises(TransientJobError) as caught:
            await CompletionClient(client, config()).submit(payload())
    assert TOKEN not in str(caught.value)
    assert caught.value.__cause__ is None


@respx.mock
async def test_completion_classifies_network_failure_as_transient() -> None:
    respx.post("https://control.example/internal/v1/jobs/job%3Atest-001/complete").mock(
        side_effect=httpx.ConnectError(f"connection failed: {TOKEN}")
    )
    async with httpx.AsyncClient() as client:
        with pytest.raises(TransientJobError) as caught:
            await CompletionClient(client, config()).submit(payload())
    assert TOKEN not in str(caught.value)
    assert caught.value.__cause__ is None


@respx.mock
async def test_completion_classifies_permanent_rejection_without_body() -> None:
    respx.post("https://control.example/internal/v1/jobs/job%3Atest-001/complete").mock(
        return_value=httpx.Response(400, text=TOKEN)
    )
    async with httpx.AsyncClient() as client:
        with pytest.raises(PermanentJobError) as caught:
            await CompletionClient(client, config()).submit(payload())
    assert "400" in str(caught.value)
    assert TOKEN not in str(caught.value)


@pytest.mark.parametrize(
    "response",
    [
        httpx.Response(200, text="not-json"),
        httpx.Response(200, json=report("run:wrong-001")),
        httpx.Response(200, json={"runId": "run:test-001"}),
    ],
)
@respx.mock
async def test_completion_rejects_malformed_or_mismatched_response(
    response: httpx.Response,
) -> None:
    respx.post("https://control.example/internal/v1/jobs/job%3Atest-001/complete").mock(
        return_value=response
    )
    async with httpx.AsyncClient() as client:
        with pytest.raises(PermanentJobError):
            await CompletionClient(client, config()).submit(payload())


def test_worker_config_requires_both_environment_values(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delenv("FLIGHTCHECK_CONTROL_PLANE_URL", raising=False)
    monkeypatch.delenv("FLIGHTCHECK_WORKER_TOKEN", raising=False)
    with pytest.raises(WorkerConfigurationError, match="required"):
        WorkerConfig.from_env()


def test_worker_config_rejects_invalid_url_without_exposing_token(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.setenv("FLIGHTCHECK_CONTROL_PLANE_URL", "not a url")
    monkeypatch.setenv("FLIGHTCHECK_WORKER_TOKEN", TOKEN)
    with pytest.raises(WorkerConfigurationError) as caught:
        WorkerConfig.from_env()
    assert TOKEN not in str(caught.value)
    assert TOKEN not in repr(config())
