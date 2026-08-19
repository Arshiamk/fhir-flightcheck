from __future__ import annotations

from datetime import datetime
from enum import StrEnum
from typing import Any, Literal

from pydantic import AnyUrl, BaseModel, ConfigDict, Field, field_validator


class ContractModel(BaseModel):
    model_config = ConfigDict(
        alias_generator=lambda value: _camel(value),
        populate_by_name=True,
        extra="forbid",
    )


def _camel(value: str) -> str:
    head, *tail = value.split("_")
    return head + "".join(part.title() for part in tail)


class Capability(StrEnum):
    NETWORK = "network"
    TARGET_CREDENTIALS = "target-credentials"
    FIXTURES = "fixtures"
    MODEL = "model"
    WRITE = "write"


class Category(StrEnum):
    FHIR = "fhir"
    RELIABILITY = "reliability"
    SECURITY = "security"
    AI_SAFETY = "ai-safety"


class Severity(StrEnum):
    INFO = "info"
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"
    CRITICAL = "critical"


class Outcome(StrEnum):
    PASS = "pass"  # noqa: S105
    FAIL = "fail"
    WARNING = "warning"
    NOT_APPLICABLE = "not_applicable"
    INCONCLUSIVE = "inconclusive"
    PLATFORM_ERROR = "platform_error"


class Rule(ContractModel):
    schema_version: Literal["1.0.0"] = "1.0.0"
    id: str = Field(pattern=r"^[a-z][a-z0-9]*(?:\.[a-z0-9]+){2,}$")
    version: str = Field(pattern=r"^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$")
    title: str = Field(min_length=4, max_length=120)
    description: str = Field(min_length=12, max_length=1000)
    category: Category
    severity: Severity
    behavior: Literal["passive", "active-read", "active-write"]
    deterministic: bool
    supported_fhir_versions: list[Literal["4.0.1"]] | None = None
    capabilities: frozenset[Capability]
    timeout_seconds: int = Field(ge=1, le=300)
    standard_references: list[AnyUrl] | None = None
    remediation: str = Field(min_length=12, max_length=2000)
    replaced_by: str | None = None
    evaluator: str = Field(pattern=r"^[a-z][a-z0-9_]{2,63}$")

    @field_validator("capabilities")
    @classmethod
    def behavior_matches_capabilities(
        cls, capabilities: frozenset[Capability], info: Any
    ) -> frozenset[Capability]:
        behavior = info.data.get("behavior")
        if behavior == "active-write" and Capability.WRITE not in capabilities:
            raise ValueError("active-write rules must declare the write capability")
        return capabilities


class Target(ContractModel):
    id: str
    base_url: AnyUrl
    fhir_version: Literal["4.0.1"]
    credential_ref: str = Field(min_length=1)
    allow_private_network: bool = False


class RunManifest(ContractModel):
    schema_version: Literal["1.0.0"] = "1.0.0"
    run_id: str
    organization_id: str
    project_id: str
    target: Target
    profile: str = Field(min_length=1)
    rule_versions: dict[str, str]
    fixture_version: str | None = None
    model_versions: dict[str, str] | None = None
    created_at: datetime


class Finding(ContractModel):
    schema_version: Literal["1.0.0"] = "1.0.0"
    finding_id: str
    run_id: str
    rule_id: str
    rule_version: str
    outcome: Outcome
    severity: Severity
    title: str = Field(min_length=1)
    summary: str = Field(min_length=1)
    evidence_refs: list[str]
    remediation: str = Field(min_length=1)
    observed_at: datetime


class Evidence(ContractModel):
    schema_version: Literal["1.0.0"] = "1.0.0"
    evidence_id: str
    run_id: str
    media_type: str = Field(min_length=1)
    sha256: str = Field(pattern=r"^[a-f0-9]{64}$")
    size_bytes: int = Field(ge=0)
    storage_uri: AnyUrl
    redaction_status: Literal["not-required", "redacted", "rejected"]
    source_rule_id: str | None = None
    created_at: datetime


class ProbeResponse(ContractModel):
    status_code: int
    headers: dict[str, str] = Field(default_factory=dict)
    body: dict[str, Any] | list[Any] | str | None = None
    elapsed_ms: int = Field(ge=0)
    error: str | None = None


class ProbeSnapshot(ContractModel):
    capability_statement: ProbeResponse | None = None
    smart_configuration: ProbeResponse | None = None
    search_bundle: ProbeResponse | None = None
    fixture: dict[str, Any] = Field(default_factory=dict)


class EvaluationResult(BaseModel):
    outcome: Outcome
    summary: str
    evidence: dict[str, Any]
