from __future__ import annotations

import asyncio
import json
import os
from collections.abc import Awaitable, Callable
from typing import Any, Literal
from urllib.parse import quote

import httpx
from pydantic import AnyHttpUrl, Field, SecretStr, ValidationError, field_validator

from .models import Capability, ContractModel, Evidence, Finding, RunManifest


class Job(ContractModel):
    job_id: str
    manifest: RunManifest
    fixture: dict[str, Any] = Field(default_factory=dict)
    capabilities: frozenset[Capability]
    attempt: int = Field(default=1, ge=1)
    max_attempts: int = Field(default=3, ge=1, le=10)


class TransientJobError(RuntimeError):
    pass


class PermanentJobError(RuntimeError):
    pass


class WorkerConfigurationError(RuntimeError):
    pass


class WorkerConfig(ContractModel):
    control_plane_url: AnyHttpUrl
    worker_token: SecretStr

    @field_validator("worker_token")
    @classmethod
    def token_must_not_be_empty(cls, token: SecretStr) -> SecretStr:
        if not token.get_secret_value().strip():
            raise ValueError("worker token must not be empty")
        return token

    @classmethod
    def from_env(cls) -> WorkerConfig:
        url = os.environ.get("FLIGHTCHECK_CONTROL_PLANE_URL")
        token = os.environ.get("FLIGHTCHECK_WORKER_TOKEN")
        if not url or not token:
            raise WorkerConfigurationError(
                "FLIGHTCHECK_CONTROL_PLANE_URL and FLIGHTCHECK_WORKER_TOKEN are required"
            )
        try:
            return cls.model_validate({"controlPlaneUrl": url, "workerToken": token})
        except ValidationError:
            raise WorkerConfigurationError("worker completion configuration is invalid") from None


class CompletionPayload(ContractModel):
    job_id: str
    run_id: str
    findings: list[Finding]
    evidence: list[Evidence]


class CompletionResponse(ContractModel):
    model_config = ContractModel.model_config | {"extra": "allow"}

    schema_version: Literal["1.0.0"]
    report_id: str
    run_id: str
    decision: Literal["ready", "conditional", "not_ready", "incomplete"]


class CompletionClient:
    def __init__(self, client: httpx.AsyncClient, config: WorkerConfig) -> None:
        self._client = client
        self._base_url = str(config.control_plane_url).rstrip("/")
        self._token = config.worker_token

    async def submit(self, payload: CompletionPayload) -> None:
        url = f"{self._base_url}/internal/v1/jobs/{quote(payload.job_id, safe='')}/complete"
        try:
            response = await self._client.post(
                url,
                headers={
                    "authorization": f"Bearer {self._token.get_secret_value()}",
                    "content-type": "application/json",
                },
                json=payload.model_dump(mode="json", by_alias=True),
            )
        except httpx.RequestError:
            raise TransientJobError("control-plane completion request failed") from None
        if response.status_code in {408, 429} or response.status_code >= 500:
            raise TransientJobError(
                f"control-plane completion temporarily unavailable ({response.status_code})"
            )
        if not 200 <= response.status_code < 300:
            raise PermanentJobError(f"control-plane completion rejected ({response.status_code})")
        try:
            completion = CompletionResponse.model_validate(response.json())
        except (ValueError, ValidationError):
            raise PermanentJobError("control-plane completion response was invalid") from None
        if completion.run_id != payload.run_id:
            raise PermanentJobError("control-plane completion response did not match the run")


JobHandler = Callable[[Job], Awaitable[None]]


class IdempotencyStore:
    def __init__(self) -> None:
        self._completed: set[str] = set()
        self._lock = asyncio.Lock()

    async def completed(self, job_id: str) -> bool:
        async with self._lock:
            return job_id in self._completed

    async def mark_completed(self, job_id: str) -> None:
        async with self._lock:
            self._completed.add(job_id)


async def process_message(
    message: Any,
    handler: JobHandler,
    store: IdempotencyStore,
) -> None:
    try:
        job = Job.model_validate_json(message.data)
    except ValueError:
        await message.term()
        return
    if await store.completed(job.job_id):
        await message.ack()
        return
    try:
        await handler(job)
    except TransientJobError:
        metadata = getattr(message, "metadata", None)
        delivery_attempt = int(getattr(metadata, "num_delivered", job.attempt))
        attempt = max(job.attempt, delivery_attempt)
        if attempt >= job.max_attempts:
            await message.term()
        else:
            await message.nak(delay=min(30.0, float(2 ** (attempt - 1))))
        return
    except Exception:
        await message.term()
        return
    await store.mark_completed(job.job_id)
    await message.ack()


async def run_consumer(handler: JobHandler) -> None:
    """Run the optional JetStream consumer when NATS configuration is supplied."""
    import nats
    from nats.errors import TimeoutError as NatsTimeoutError

    server = os.environ.get("NATS_URL", "nats://127.0.0.1:4222")
    subject = os.environ.get("FLIGHTCHECK_JOB_SUBJECT", "flightcheck.jobs.evaluate")
    durable = os.environ.get("FLIGHTCHECK_DURABLE", "evaluator")
    connection = await nats.connect(server)
    jetstream = connection.jetstream()
    subscription = await jetstream.pull_subscribe(subject, durable=durable)
    store = IdempotencyStore()
    try:
        while True:
            try:
                messages = await subscription.fetch(batch=8, timeout=5)
            except NatsTimeoutError:
                continue
            await asyncio.gather(
                *(process_message(message, handler, store) for message in messages)
            )
    finally:
        await connection.drain()


def encode_job(job: Job) -> bytes:
    return json.dumps(job.model_dump(mode="json", by_alias=True), sort_keys=True).encode()
