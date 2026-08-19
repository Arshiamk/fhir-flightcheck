from __future__ import annotations

import argparse
import asyncio
import json
from pathlib import Path
from typing import Any

import httpx
from pydantic import AnyUrl

from .evaluator import Evaluator, HttpProber
from .models import Capability, Outcome, ProbeSnapshot, RunManifest
from .registry import RuleRegistry
from .rules import evaluator_map
from .storage import ArtifactStore, make_artifact_store
from .worker import (
    CompletionClient,
    CompletionPayload,
    Job,
    WorkerConfig,
    run_consumer,
)

_SARIF_LEVEL: dict[Outcome, str] = {
    Outcome.FAIL: "error",
    Outcome.WARNING: "warning",
    Outcome.INCONCLUSIVE: "note",
    Outcome.PASS: "none",
    Outcome.NOT_APPLICABLE: "none",
    Outcome.PLATFORM_ERROR: "note",
}


def _default_pack_path() -> Path:
    return Path(__file__).resolve().parents[4] / "packages" / "rule-packs"


def build_registry(pack_path: Path) -> RuleRegistry:
    registry = RuleRegistry(evaluator_map())
    registry.load_directory(pack_path)
    return registry


async def evaluate_payload(
    manifest: RunManifest,
    fixture: dict[str, Any],
    capabilities: frozenset[Capability],
    pack_path: Path,
    client: httpx.AsyncClient | None = None,
    store: ArtifactStore | None = None,
) -> dict[str, Any]:
    registry = build_registry(pack_path)
    snapshot = ProbeSnapshot(fixture=fixture)
    if Capability.NETWORK in capabilities:
        if client is None:
            timeout = httpx.Timeout(20.0, connect=5.0)
            async with httpx.AsyncClient(timeout=timeout) as owned_client:
                snapshot = await HttpProber(owned_client).collect(manifest, fixture)
        else:
            snapshot = await HttpProber(client).collect(manifest, fixture)
    findings, artifacts = Evaluator(registry).evaluate(manifest, snapshot, capabilities)

    effective_store: ArtifactStore = store if store is not None else make_artifact_store()
    uploaded_evidence = []
    for artifact in artifacts:
        uri = await effective_store.upload(artifact)
        updated_metadata = artifact.metadata.model_copy(update={"storage_uri": AnyUrl(uri)})
        uploaded_evidence.append(updated_metadata)

    return {
        "findings": [finding.model_dump(mode="json", by_alias=True) for finding in findings],
        "evidence": [ev.model_dump(mode="json", by_alias=True) for ev in uploaded_evidence],
    }


def _to_sarif(result: dict[str, Any], pack_path: Path) -> dict[str, Any]:
    registry = build_registry(pack_path)
    rules_by_id = {rule.id: rule for rule in registry.rules}

    findings = result["findings"]
    sarif_rules: list[dict[str, Any]] = []
    seen_rule_ids: set[str] = set()
    for finding in findings:
        rule_id = finding["ruleId"]
        if rule_id in seen_rule_ids:
            continue
        seen_rule_ids.add(rule_id)
        rule = rules_by_id.get(rule_id)
        sarif_rules.append(
            {
                "id": rule_id,
                "name": rule.title if rule else rule_id,
                "shortDescription": {"text": rule.title if rule else rule_id},
                "help": {"text": rule.remediation if rule else ""},
            }
        )

    sarif_results: list[dict[str, Any]] = []
    for finding in findings:
        try:
            outcome_enum = Outcome(finding.get("outcome", "inconclusive"))
        except ValueError:
            outcome_enum = Outcome.INCONCLUSIVE
        level = _SARIF_LEVEL.get(outcome_enum, "note")
        sarif_results.append(
            {
                "ruleId": finding["ruleId"],
                "level": level,
                "message": {"text": finding.get("summary", "")},
            }
        )

    return {
        "version": "2.1.0",
        "$schema": "https://json.schemastore.org/sarif-2.1.0.json",
        "runs": [
            {
                "tool": {
                    "driver": {
                        "name": "FHIR Flightcheck",
                        "version": "1.0.0",
                        "rules": sarif_rules,
                    }
                },
                "results": sarif_results,
            }
        ],
    }


async def _run_once(args: argparse.Namespace) -> int:
    manifest = RunManifest.model_validate_json(args.manifest.read_text(encoding="utf-8"))
    fixture = json.loads(args.fixture.read_text(encoding="utf-8")) if args.fixture else {}
    capabilities = frozenset(Capability(item) for item in args.capability)
    result = await evaluate_payload(manifest, fixture, capabilities, args.packs)
    output_format: str = getattr(args, "format", "json")
    if output_format == "sarif":
        rendered = json.dumps(_to_sarif(result, args.packs), indent=2, sort_keys=True)
    else:
        rendered = json.dumps(result, indent=2, sort_keys=True)
    if args.output:
        args.output.write_text(rendered + "\n", encoding="utf-8")
    else:
        print(rendered)
    return 0


async def _run_worker(args: argparse.Namespace) -> int:
    config = WorkerConfig.from_env()
    store = make_artifact_store()
    timeout = httpx.Timeout(20.0, connect=5.0, read=15.0, write=15.0, pool=5.0)
    limits = httpx.Limits(max_connections=16, max_keepalive_connections=8)
    async with httpx.AsyncClient(timeout=timeout, limits=limits, follow_redirects=False) as client:
        completion_client = CompletionClient(client, config)

        async def handler(job: Job) -> None:
            result = await evaluate_payload(
                job.manifest,
                job.fixture,
                job.capabilities,
                args.packs,
                client,
                store,
            )
            payload = CompletionPayload.model_validate(
                {
                    "jobId": job.job_id,
                    "runId": job.manifest.run_id,
                    "findings": result["findings"],
                    "evidence": result["evidence"],
                }
            )
            await completion_client.submit(payload)

        await run_consumer(handler)
    return 0


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="FHIR Flightcheck deterministic evaluator")
    parser.add_argument("--packs", type=Path, default=_default_pack_path())
    subparsers = parser.add_subparsers(dest="command", required=True)
    once = subparsers.add_parser("evaluate", help="evaluate one manifest")
    once.add_argument("--manifest", required=True, type=Path)
    once.add_argument("--fixture", type=Path)
    once.add_argument("--output", type=Path)
    once.add_argument(
        "--capability",
        action="append",
        choices=[item.value for item in Capability],
        default=[],
    )
    once.add_argument(
        "--format",
        choices=["json", "sarif"],
        default="json",
        help="output format (default: json)",
    )
    subparsers.add_parser("worker", help="consume JetStream jobs")
    return parser


def main() -> int:
    args = _parser().parse_args()
    if args.command == "evaluate":
        return asyncio.run(_run_once(args))
    return asyncio.run(_run_worker(args))
