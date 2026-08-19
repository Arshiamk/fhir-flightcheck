from __future__ import annotations

import json
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

import pytest
from pydantic import ValidationError

from flightcheck_evaluator import cli
from flightcheck_evaluator.evaluator import HttpProber
from flightcheck_evaluator.models import Capability, ProbeSnapshot, RunManifest
from flightcheck_evaluator.worker import (
    CompletionClient,
    CompletionPayload,
    Job,
    WorkerConfigurationError,
)

ROOT = Path(__file__).resolve().parents[3]
PACKS = ROOT / "packages" / "rule-packs"
HEALTHY = ROOT / "fixtures" / "synthea" / "healthy.json"


def manifest_payload(rule_id: str) -> dict[str, Any]:
    return {
        "schemaVersion": "1.0.0",
        "runId": "run:cli-001",
        "organizationId": "org:cli-001",
        "projectId": "project:cli-001",
        "target": {
            "id": "target:cli-001",
            "baseUrl": "https://synthetic.example/fhir",
            "fhirVersion": "4.0.1",
            "credentialRef": "none",
        },
        "profile": "test",
        "ruleVersions": {rule_id: "1.0.0"},
        "createdAt": datetime(2026, 8, 18, tzinfo=UTC).isoformat(),
    }


def write_manifest(tmp_path: Path, rule_id: str = "fhir.narrative.safety") -> Path:
    path = tmp_path / "manifest.json"
    path.write_text(json.dumps(manifest_payload(rule_id)), encoding="utf-8")
    return path


def cli_args(manifest: Path, *extra: str) -> list[str]:
    return [
        "flightcheck-evaluator",
        "--packs",
        str(PACKS),
        "evaluate",
        "--manifest",
        str(manifest),
        *extra,
    ]


def test_cli_success_prints_contract_json(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    monkeypatch.setattr(
        sys,
        "argv",
        cli_args(
            write_manifest(tmp_path),
            "--fixture",
            str(HEALTHY),
            "--capability",
            "fixtures",
        ),
    )
    assert cli.main() == 0
    output = json.loads(capsys.readouterr().out)
    assert output["findings"][0]["ruleId"] == "fhir.narrative.safety"
    assert output["findings"][0]["outcome"] == "pass"
    assert output["evidence"][0]["sha256"]


def test_cli_writes_json_output_file(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
) -> None:
    output_path = tmp_path / "result.json"
    monkeypatch.setattr(
        sys,
        "argv",
        cli_args(
            write_manifest(tmp_path),
            "--fixture",
            str(HEALTHY),
            "--capability",
            "fixtures",
            "--output",
            str(output_path),
        ),
    )
    assert cli.main() == 0
    assert capsys.readouterr().out == ""
    output = json.loads(output_path.read_text(encoding="utf-8"))
    assert len(output["findings"]) == 1
    assert output_path.read_text(encoding="utf-8").endswith("\n")


def test_cli_rejects_invalid_manifest(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    path = tmp_path / "invalid.json"
    path.write_text("{}", encoding="utf-8")
    monkeypatch.setattr(sys, "argv", cli_args(path))
    with pytest.raises(ValidationError):
        cli.main()


async def test_evaluate_payload_collects_network_probes(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    manifest = RunManifest.model_validate(manifest_payload("fhir.discovery.capability"))
    fixture = json.loads(HEALTHY.read_text(encoding="utf-8"))
    probes = fixture["probes"]
    expected = ProbeSnapshot.model_validate(
        {
            "capabilityStatement": probes["capabilityStatement"],
            "smartConfiguration": probes["smartConfiguration"],
            "searchBundle": probes["searchBundle"],
            "fixture": fixture,
        }
    )
    collected = False

    async def collect(
        _self: object, _manifest: RunManifest, supplied: dict[str, Any]
    ) -> ProbeSnapshot:
        nonlocal collected
        collected = True
        assert supplied == fixture
        return expected

    monkeypatch.setattr(HttpProber, "collect", collect)
    result = await cli.evaluate_payload(manifest, fixture, frozenset({Capability.NETWORK}), PACKS)
    assert collected
    assert result["findings"][0]["outcome"] == "pass"


async def test_worker_entrypoint_evaluates_consumed_job(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    handled = False
    submitted: CompletionPayload | None = None

    async def consume(handler: Any) -> None:
        nonlocal handled
        payload = manifest_payload("fhir.narrative.safety")
        job = Job.model_validate(
            {
                "jobId": "job:cli-001",
                "manifest": payload,
                "fixture": json.loads(HEALTHY.read_text(encoding="utf-8")),
                "capabilities": ["fixtures"],
            }
        )
        await handler(job)
        handled = True

    async def submit(_self: CompletionClient, payload: CompletionPayload) -> None:
        nonlocal submitted
        submitted = payload

    monkeypatch.setenv("FLIGHTCHECK_CONTROL_PLANE_URL", "https://control.example")
    monkeypatch.setenv("FLIGHTCHECK_WORKER_TOKEN", "worker-token-long-enough-for-tests")
    monkeypatch.setattr(cli, "run_consumer", consume)
    monkeypatch.setattr(CompletionClient, "submit", submit)
    args = cli._parser().parse_args(["--packs", str(PACKS), "worker"])
    assert await cli._run_worker(args) == 0
    assert handled
    assert submitted is not None
    assert submitted.job_id == "job:cli-001"
    assert submitted.run_id == "run:cli-001"
    assert len(submitted.findings) == 1
    assert len(submitted.evidence) == 1


async def test_worker_startup_rejects_missing_configuration(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    monkeypatch.delenv("FLIGHTCHECK_CONTROL_PLANE_URL", raising=False)
    monkeypatch.delenv("FLIGHTCHECK_WORKER_TOKEN", raising=False)
    args = cli._parser().parse_args(["--packs", str(PACKS), "worker"])
    with pytest.raises(WorkerConfigurationError, match="required"):
        await cli._run_worker(args)
