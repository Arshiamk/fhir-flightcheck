from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from pydantic import AnyUrl

from .models import Evidence
from .redaction import redact


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"), sort_keys=True).encode(
        "utf-8"
    )


@dataclass(frozen=True)
class EvidenceArtifact:
    metadata: Evidence
    content: bytes


def build_evidence(
    *,
    run_id: str,
    rule_id: str,
    value: Any,
    created_at: datetime | None = None,
) -> EvidenceArtifact:
    redaction = redact(value)
    content = canonical_json(redaction.value)
    digest = hashlib.sha256(content).hexdigest()
    evidence_id = f"ev:{digest[:24]}"
    metadata = Evidence(
        evidence_id=evidence_id,
        run_id=run_id,
        media_type="application/json",
        sha256=digest,
        size_bytes=len(content),
        storage_uri=AnyUrl(f"urn:sha256:{digest}"),
        redaction_status="redacted" if redaction.changed else "not-required",
        source_rule_id=rule_id,
        created_at=created_at or datetime.now(UTC),
    )
    return EvidenceArtifact(metadata=metadata, content=content)
