from __future__ import annotations

import asyncio
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

import nats
import pytest
from nats.errors import TimeoutError as NatsTimeoutError

from flightcheck_evaluator.worker import (
    IdempotencyStore,
    Job,
    TransientJobError,
    encode_job,
    process_message,
    run_consumer,
)


@dataclass
class Metadata:
    num_delivered: int


class FakeMessage:
    def __init__(self, data: bytes, delivered: int = 1) -> None:
        self.data = data
        self.metadata: Metadata | None = Metadata(delivered)
        self.acked = False
        self.terminated = False
        self.delay: float | None = None

    async def ack(self) -> None:
        self.acked = True

    async def nak(self, delay: float | None = None) -> None:
        self.delay = delay

    async def term(self) -> None:
        self.terminated = True


def job(max_attempts: int = 3, attempt: int = 1) -> Job:
    return Job.model_validate(
        {
            "jobId": "job:test-001",
            "manifest": {
                "runId": "run:worker-001",
                "organizationId": "org:worker-001",
                "projectId": "project:worker-001",
                "target": {
                    "id": "target:worker-001",
                    "baseUrl": "https://synthetic.example/fhir",
                    "fhirVersion": "4.0.1",
                    "credentialRef": "none",
                },
                "profile": "test",
                "ruleVersions": {"fhir.discovery.capability": "1.0.0"},
                "createdAt": datetime(2026, 8, 18, tzinfo=UTC).isoformat(),
            },
            "capabilities": ["network"],
            "attempt": attempt,
            "maxAttempts": max_attempts,
        }
    )


async def test_duplicate_completion_is_acknowledged_without_reexecution() -> None:
    store = IdempotencyStore()
    calls = 0

    async def handler(_: Job) -> None:
        nonlocal calls
        calls += 1

    first = FakeMessage(encode_job(job()))
    duplicate = FakeMessage(encode_job(job()), delivered=2)
    await process_message(first, handler, store)
    await process_message(duplicate, handler, store)
    assert first.acked and duplicate.acked
    assert calls == 1


async def test_transient_failure_retries_with_bounded_backoff() -> None:
    async def handler(_: Job) -> None:
        raise TransientJobError

    message = FakeMessage(encode_job(job()), delivered=2)
    await process_message(message, handler, IdempotencyStore())
    assert message.delay == 2.0
    assert not message.terminated


async def test_exhausted_transient_failure_is_terminated() -> None:
    async def handler(_: Job) -> None:
        raise TransientJobError

    message = FakeMessage(encode_job(job(max_attempts=3)), delivered=3)
    await process_message(message, handler, IdempotencyStore())
    assert message.terminated
    assert message.delay is None


async def test_malformed_job_is_terminated() -> None:
    async def handler(_: Job) -> None:
        raise AssertionError("handler must not run")

    message = FakeMessage(b"not-json")
    await process_message(message, handler, IdempotencyStore())
    assert message.terminated


async def test_permanent_handler_error_is_dead_lettered() -> None:
    async def handler(_: Job) -> None:
        raise RuntimeError("permanent")

    message = FakeMessage(encode_job(job()))
    await process_message(message, handler, IdempotencyStore())
    assert message.terminated
    assert not message.acked


async def test_payload_attempt_bounds_retry_without_metadata() -> None:
    async def handler(_: Job) -> None:
        raise TransientJobError

    message = FakeMessage(encode_job(job(max_attempts=10, attempt=6)))
    message.metadata = None
    await process_message(message, handler, IdempotencyStore())
    assert message.delay == 30.0


async def test_consumer_survives_idle_timeout_and_drains(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    class Subscription:
        def __init__(self) -> None:
            self.calls = 0

        async def fetch(self, **_: Any) -> list[Any]:
            self.calls += 1
            if self.calls == 1:
                raise NatsTimeoutError
            if self.calls == 2:
                return []
            raise asyncio.CancelledError

    class JetStream:
        def __init__(self, subscription: Subscription) -> None:
            self.subscription = subscription

        async def pull_subscribe(self, subject: str, durable: str) -> Subscription:
            assert subject and durable
            return self.subscription

    class Connection:
        def __init__(self) -> None:
            self.subscription = Subscription()
            self.drained = False

        def jetstream(self) -> JetStream:
            return JetStream(self.subscription)

        async def drain(self) -> None:
            self.drained = True

    connection = Connection()

    async def connect(_: str) -> Connection:
        return connection

    async def handler(_: Job) -> None:
        pass

    monkeypatch.setattr(nats, "connect", connect)
    with pytest.raises(asyncio.CancelledError):
        await run_consumer(handler)
    assert connection.subscription.calls == 3
    assert connection.drained
