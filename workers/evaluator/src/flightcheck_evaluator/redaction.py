from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Any

SENSITIVE_KEYS = frozenset(
    {
        "access_token",
        "authorization",
        "birthDate",
        "client_secret",
        "family",
        "given",
        "identifier",
        "name",
        "patient",
        "refresh_token",
        "telecom",
        "text",
    }
)
TOKEN_PATTERN = re.compile(r"(?i)\b(?:bearer\s+)?[A-Za-z0-9_-]{20,}\b")
EMAIL_PATTERN = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")


@dataclass(frozen=True)
class RedactionResult:
    value: Any
    changed: bool


def redact(value: Any) -> RedactionResult:
    """Recursively redact likely secrets and direct identifiers."""
    if isinstance(value, dict):
        changed = False
        output: dict[str, Any] = {}
        for key, child in value.items():
            if key.lower() in {item.lower() for item in SENSITIVE_KEYS}:
                output[key] = "[REDACTED]"
                changed = True
            else:
                result = redact(child)
                output[key] = result.value
                changed = changed or result.changed
        return RedactionResult(output, changed)
    if isinstance(value, list):
        results = [redact(item) for item in value]
        return RedactionResult(
            [item.value for item in results], any(item.changed for item in results)
        )
    if isinstance(value, str):
        redacted = EMAIL_PATTERN.sub("[REDACTED_EMAIL]", value)
        redacted = TOKEN_PATTERN.sub("[REDACTED_TOKEN]", redacted)
        return RedactionResult(redacted, redacted != value)
    return RedactionResult(value, False)


def contains_sensitive_material(value: Any) -> bool:
    result = redact(value)
    return result.changed
