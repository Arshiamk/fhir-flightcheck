from enum import Enum
from dataclasses import dataclass
from typing import List, Optional, Any, Dict, TypeVar, Callable, Type, cast
from datetime import datetime
import dateutil.parser


T = TypeVar("T")
EnumT = TypeVar("EnumT", bound=Enum)


def from_list(f: Callable[[Any], T], x: Any) -> List[T]:
    assert isinstance(x, list)
    return [f(y) for y in x]


def from_str(x: Any) -> str:
    assert isinstance(x, str)
    return x


def from_bool(x: Any) -> bool:
    assert isinstance(x, bool)
    return x


def from_int(x: Any) -> int:
    assert isinstance(x, int) and not isinstance(x, bool)
    return x


def from_none(x: Any) -> Any:
    assert x is None
    return x


def from_union(fs, x):
    for f in fs:
        try:
            return f(x)
        except:
            pass
    assert False


def to_enum(c: Type[EnumT], x: Any) -> EnumT:
    assert isinstance(x, c)
    return x.value


def from_datetime(x: Any) -> datetime:
    return dateutil.parser.parse(x)


def from_dict(f: Callable[[Any], T], x: Any) -> Dict[str, T]:
    assert isinstance(x, dict)
    return { k: f(v) for (k, v) in x.items() }


def to_class(c: Type[T], x: Any) -> dict:
    assert isinstance(x, c)
    return cast(Any, x).to_dict()


class Behavior(Enum):
    ACTIVE_READ = "active-read"
    ACTIVE_WRITE = "active-write"
    PASSIVE = "passive"


class Capability(Enum):
    FIXTURES = "fixtures"
    MODEL = "model"
    NETWORK = "network"
    TARGET_CREDENTIALS = "target-credentials"
    WRITE = "write"


class RuleCategory(Enum):
    AI_SAFETY = "ai-safety"
    FHIR = "fhir"
    RELIABILITY = "reliability"
    SECURITY = "security"


class SchemaVersion(Enum):
    THE_100 = "1.0.0"


class Severity(Enum):
    CRITICAL = "critical"
    HIGH = "high"
    INFO = "info"
    LOW = "low"
    MEDIUM = "medium"


class FhirVersion(Enum):
    THE_401 = "4.0.1"


@dataclass
class Rule:
    behavior: Behavior
    capabilities: List[Capability]
    category: RuleCategory
    description: str
    deterministic: bool
    evaluator: str
    id: str
    remediation: str
    schema_version: SchemaVersion
    severity: Severity
    timeout_seconds: int
    title: str
    version: str
    replaced_by: Optional[str] = None
    standard_references: Optional[List[str]] = None
    supported_fhir_versions: Optional[List[FhirVersion]] = None

    @staticmethod
    def from_dict(obj: Any) -> 'Rule':
        assert isinstance(obj, dict)
        behavior = Behavior(obj.get("behavior"))
        capabilities = from_list(Capability, obj.get("capabilities"))
        category = RuleCategory(obj.get("category"))
        description = from_str(obj.get("description"))
        deterministic = from_bool(obj.get("deterministic"))
        evaluator = from_str(obj.get("evaluator"))
        id = from_str(obj.get("id"))
        remediation = from_str(obj.get("remediation"))
        schema_version = SchemaVersion(obj.get("schemaVersion"))
        severity = Severity(obj.get("severity"))
        timeout_seconds = from_int(obj.get("timeoutSeconds"))
        title = from_str(obj.get("title"))
        version = from_str(obj.get("version"))
        replaced_by = from_union([from_str, from_none], obj.get("replacedBy"))
        standard_references = from_union([lambda x: from_list(from_str, x), from_none], obj.get("standardReferences"))
        supported_fhir_versions = from_union([lambda x: from_list(FhirVersion, x), from_none], obj.get("supportedFhirVersions"))
        return Rule(behavior, capabilities, category, description, deterministic, evaluator, id, remediation, schema_version, severity, timeout_seconds, title, version, replaced_by, standard_references, supported_fhir_versions)

    def to_dict(self) -> dict:
        result: dict = {}
        result["behavior"] = to_enum(Behavior, self.behavior)
        result["capabilities"] = from_list(lambda x: to_enum(Capability, x), self.capabilities)
        result["category"] = to_enum(RuleCategory, self.category)
        result["description"] = from_str(self.description)
        result["deterministic"] = from_bool(self.deterministic)
        result["evaluator"] = from_str(self.evaluator)
        result["id"] = from_str(self.id)
        result["remediation"] = from_str(self.remediation)
        result["schemaVersion"] = to_enum(SchemaVersion, self.schema_version)
        result["severity"] = to_enum(Severity, self.severity)
        result["timeoutSeconds"] = from_int(self.timeout_seconds)
        result["title"] = from_str(self.title)
        result["version"] = from_str(self.version)
        if self.replaced_by is not None:
            result["replacedBy"] = from_union([from_str, from_none], self.replaced_by)
        if self.standard_references is not None:
            result["standardReferences"] = from_union([lambda x: from_list(from_str, x), from_none], self.standard_references)
        if self.supported_fhir_versions is not None:
            result["supportedFhirVersions"] = from_union([lambda x: from_list(lambda x: to_enum(FhirVersion, x), x), from_none], self.supported_fhir_versions)
        return result


@dataclass
class Target:
    base_url: str
    credential_ref: str
    fhir_version: FhirVersion
    id: str
    allow_private_network: Optional[bool] = None

    @staticmethod
    def from_dict(obj: Any) -> 'Target':
        assert isinstance(obj, dict)
        base_url = from_str(obj.get("baseUrl"))
        credential_ref = from_str(obj.get("credentialRef"))
        fhir_version = FhirVersion(obj.get("fhirVersion"))
        id = from_str(obj.get("id"))
        allow_private_network = from_union([from_bool, from_none], obj.get("allowPrivateNetwork"))
        return Target(base_url, credential_ref, fhir_version, id, allow_private_network)

    def to_dict(self) -> dict:
        result: dict = {}
        result["baseUrl"] = from_str(self.base_url)
        result["credentialRef"] = from_str(self.credential_ref)
        result["fhirVersion"] = to_enum(FhirVersion, self.fhir_version)
        result["id"] = from_str(self.id)
        if self.allow_private_network is not None:
            result["allowPrivateNetwork"] = from_union([from_bool, from_none], self.allow_private_network)
        return result


@dataclass
class RunManifest:
    created_at: datetime
    organization_id: str
    profile: str
    project_id: str
    rule_versions: Dict[str, str]
    run_id: str
    schema_version: SchemaVersion
    target: Target
    fixture_version: Optional[str] = None
    model_versions: Optional[Dict[str, str]] = None

    @staticmethod
    def from_dict(obj: Any) -> 'RunManifest':
        assert isinstance(obj, dict)
        created_at = from_datetime(obj.get("createdAt"))
        organization_id = from_str(obj.get("organizationId"))
        profile = from_str(obj.get("profile"))
        project_id = from_str(obj.get("projectId"))
        rule_versions = from_dict(from_str, obj.get("ruleVersions"))
        run_id = from_str(obj.get("runId"))
        schema_version = SchemaVersion(obj.get("schemaVersion"))
        target = Target.from_dict(obj.get("target"))
        fixture_version = from_union([from_str, from_none], obj.get("fixtureVersion"))
        model_versions = from_union([lambda x: from_dict(from_str, x), from_none], obj.get("modelVersions"))
        return RunManifest(created_at, organization_id, profile, project_id, rule_versions, run_id, schema_version, target, fixture_version, model_versions)

    def to_dict(self) -> dict:
        result: dict = {}
        result["createdAt"] = self.created_at.isoformat()
        result["organizationId"] = from_str(self.organization_id)
        result["profile"] = from_str(self.profile)
        result["projectId"] = from_str(self.project_id)
        result["ruleVersions"] = from_dict(from_str, self.rule_versions)
        result["runId"] = from_str(self.run_id)
        result["schemaVersion"] = to_enum(SchemaVersion, self.schema_version)
        result["target"] = to_class(Target, self.target)
        if self.fixture_version is not None:
            result["fixtureVersion"] = from_union([from_str, from_none], self.fixture_version)
        if self.model_versions is not None:
            result["modelVersions"] = from_union([lambda x: from_dict(from_str, x), from_none], self.model_versions)
        return result


class Outcome(Enum):
    FAIL = "fail"
    INCONCLUSIVE = "inconclusive"
    NOT_APPLICABLE = "not_applicable"
    PASS = "pass"
    PLATFORM_ERROR = "platform_error"
    WARNING = "warning"


@dataclass
class Finding:
    evidence_refs: List[str]
    finding_id: str
    observed_at: datetime
    outcome: Outcome
    remediation: str
    rule_id: str
    rule_version: str
    run_id: str
    schema_version: SchemaVersion
    severity: Severity
    summary: str
    title: str

    @staticmethod
    def from_dict(obj: Any) -> 'Finding':
        assert isinstance(obj, dict)
        evidence_refs = from_list(from_str, obj.get("evidenceRefs"))
        finding_id = from_str(obj.get("findingId"))
        observed_at = from_datetime(obj.get("observedAt"))
        outcome = Outcome(obj.get("outcome"))
        remediation = from_str(obj.get("remediation"))
        rule_id = from_str(obj.get("ruleId"))
        rule_version = from_str(obj.get("ruleVersion"))
        run_id = from_str(obj.get("runId"))
        schema_version = SchemaVersion(obj.get("schemaVersion"))
        severity = Severity(obj.get("severity"))
        summary = from_str(obj.get("summary"))
        title = from_str(obj.get("title"))
        return Finding(evidence_refs, finding_id, observed_at, outcome, remediation, rule_id, rule_version, run_id, schema_version, severity, summary, title)

    def to_dict(self) -> dict:
        result: dict = {}
        result["evidenceRefs"] = from_list(from_str, self.evidence_refs)
        result["findingId"] = from_str(self.finding_id)
        result["observedAt"] = self.observed_at.isoformat()
        result["outcome"] = to_enum(Outcome, self.outcome)
        result["remediation"] = from_str(self.remediation)
        result["ruleId"] = from_str(self.rule_id)
        result["ruleVersion"] = from_str(self.rule_version)
        result["runId"] = from_str(self.run_id)
        result["schemaVersion"] = to_enum(SchemaVersion, self.schema_version)
        result["severity"] = to_enum(Severity, self.severity)
        result["summary"] = from_str(self.summary)
        result["title"] = from_str(self.title)
        return result


class RedactionStatus(Enum):
    NOT_REQUIRED = "not-required"
    REDACTED = "redacted"
    REJECTED = "rejected"


@dataclass
class Evidence:
    created_at: datetime
    evidence_id: str
    media_type: str
    redaction_status: RedactionStatus
    run_id: str
    schema_version: SchemaVersion
    sha256: str
    size_bytes: int
    storage_uri: str
    source_rule_id: Optional[str] = None

    @staticmethod
    def from_dict(obj: Any) -> 'Evidence':
        assert isinstance(obj, dict)
        created_at = from_datetime(obj.get("createdAt"))
        evidence_id = from_str(obj.get("evidenceId"))
        media_type = from_str(obj.get("mediaType"))
        redaction_status = RedactionStatus(obj.get("redactionStatus"))
        run_id = from_str(obj.get("runId"))
        schema_version = SchemaVersion(obj.get("schemaVersion"))
        sha256 = from_str(obj.get("sha256"))
        size_bytes = from_int(obj.get("sizeBytes"))
        storage_uri = from_str(obj.get("storageUri"))
        source_rule_id = from_union([from_str, from_none], obj.get("sourceRuleId"))
        return Evidence(created_at, evidence_id, media_type, redaction_status, run_id, schema_version, sha256, size_bytes, storage_uri, source_rule_id)

    def to_dict(self) -> dict:
        result: dict = {}
        result["createdAt"] = self.created_at.isoformat()
        result["evidenceId"] = from_str(self.evidence_id)
        result["mediaType"] = from_str(self.media_type)
        result["redactionStatus"] = to_enum(RedactionStatus, self.redaction_status)
        result["runId"] = from_str(self.run_id)
        result["schemaVersion"] = to_enum(SchemaVersion, self.schema_version)
        result["sha256"] = from_str(self.sha256)
        result["sizeBytes"] = from_int(self.size_bytes)
        result["storageUri"] = from_str(self.storage_uri)
        if self.source_rule_id is not None:
            result["sourceRuleId"] = from_union([from_str, from_none], self.source_rule_id)
        return result


@dataclass
class Coverage:
    completed: int
    selected: int

    @staticmethod
    def from_dict(obj: Any) -> 'Coverage':
        assert isinstance(obj, dict)
        completed = from_int(obj.get("completed"))
        selected = from_int(obj.get("selected"))
        return Coverage(completed, selected)

    def to_dict(self) -> dict:
        result: dict = {}
        result["completed"] = from_int(self.completed)
        result["selected"] = from_int(self.selected)
        return result


class Decision(Enum):
    CONDITIONAL = "conditional"
    INCOMPLETE = "incomplete"
    NOT_READY = "not_ready"
    READY = "ready"


@dataclass
class ReportSchema:
    evidence_refs: List[str]
    finding_id: str
    observed_at: datetime
    outcome: Outcome
    remediation: str
    rule_id: str
    rule_version: str
    run_id: str
    schema_version: SchemaVersion
    severity: Severity
    summary: str
    title: str

    @staticmethod
    def from_dict(obj: Any) -> 'ReportSchema':
        assert isinstance(obj, dict)
        evidence_refs = from_list(from_str, obj.get("evidenceRefs"))
        finding_id = from_str(obj.get("findingId"))
        observed_at = from_datetime(obj.get("observedAt"))
        outcome = Outcome(obj.get("outcome"))
        remediation = from_str(obj.get("remediation"))
        rule_id = from_str(obj.get("ruleId"))
        rule_version = from_str(obj.get("ruleVersion"))
        run_id = from_str(obj.get("runId"))
        schema_version = SchemaVersion(obj.get("schemaVersion"))
        severity = Severity(obj.get("severity"))
        summary = from_str(obj.get("summary"))
        title = from_str(obj.get("title"))
        return ReportSchema(evidence_refs, finding_id, observed_at, outcome, remediation, rule_id, rule_version, run_id, schema_version, severity, summary, title)

    def to_dict(self) -> dict:
        result: dict = {}
        result["evidenceRefs"] = from_list(from_str, self.evidence_refs)
        result["findingId"] = from_str(self.finding_id)
        result["observedAt"] = self.observed_at.isoformat()
        result["outcome"] = to_enum(Outcome, self.outcome)
        result["remediation"] = from_str(self.remediation)
        result["ruleId"] = from_str(self.rule_id)
        result["ruleVersion"] = from_str(self.rule_version)
        result["runId"] = from_str(self.run_id)
        result["schemaVersion"] = to_enum(SchemaVersion, self.schema_version)
        result["severity"] = to_enum(Severity, self.severity)
        result["summary"] = from_str(self.summary)
        result["title"] = from_str(self.title)
        return result


class Algorithm(Enum):
    ED25519 = "Ed25519"


@dataclass
class Signature:
    algorithm: Algorithm
    key_id: str
    value: str

    @staticmethod
    def from_dict(obj: Any) -> 'Signature':
        assert isinstance(obj, dict)
        algorithm = Algorithm(obj.get("algorithm"))
        key_id = from_str(obj.get("keyId"))
        value = from_str(obj.get("value"))
        return Signature(algorithm, key_id, value)

    def to_dict(self) -> dict:
        result: dict = {}
        result["algorithm"] = to_enum(Algorithm, self.algorithm)
        result["keyId"] = from_str(self.key_id)
        result["value"] = from_str(self.value)
        return result


@dataclass
class Report:
    coverage: Coverage
    created_at: datetime
    decision: Decision
    findings: List[ReportSchema]
    manifest_sha256: str
    report_id: str
    run_id: str
    schema_version: SchemaVersion
    signature: Optional[Signature] = None

    @staticmethod
    def from_dict(obj: Any) -> 'Report':
        assert isinstance(obj, dict)
        coverage = Coverage.from_dict(obj.get("coverage"))
        created_at = from_datetime(obj.get("createdAt"))
        decision = Decision(obj.get("decision"))
        findings = from_list(ReportSchema.from_dict, obj.get("findings"))
        manifest_sha256 = from_str(obj.get("manifestSha256"))
        report_id = from_str(obj.get("reportId"))
        run_id = from_str(obj.get("runId"))
        schema_version = SchemaVersion(obj.get("schemaVersion"))
        signature = from_union([Signature.from_dict, from_none], obj.get("signature"))
        return Report(coverage, created_at, decision, findings, manifest_sha256, report_id, run_id, schema_version, signature)

    def to_dict(self) -> dict:
        result: dict = {}
        result["coverage"] = to_class(Coverage, self.coverage)
        result["createdAt"] = self.created_at.isoformat()
        result["decision"] = to_enum(Decision, self.decision)
        result["findings"] = from_list(lambda x: to_class(ReportSchema, x), self.findings)
        result["manifestSha256"] = from_str(self.manifest_sha256)
        result["reportId"] = from_str(self.report_id)
        result["runId"] = from_str(self.run_id)
        result["schemaVersion"] = to_enum(SchemaVersion, self.schema_version)
        if self.signature is not None:
            result["signature"] = from_union([lambda x: to_class(Signature, x), from_none], self.signature)
        return result


class APIProblemCategory(Enum):
    AUTHORIZATION = "authorization"
    CONFIGURATION = "configuration"
    PERMANENT_TARGET = "permanent_target"
    PLATFORM = "platform"
    RULE_DEFECT = "rule_defect"
    TARGET = "target"
    TRANSIENT_TARGET = "transient_target"


@dataclass
class APIProblem:
    category: APIProblemCategory
    code: str
    detail: str
    instance: str
    retryable: bool
    schema_version: SchemaVersion
    status: int
    title: str
    trace_id: str
    type: str
    retry_after_seconds: Optional[int] = None
    run_id: Optional[str] = None

    @staticmethod
    def from_dict(obj: Any) -> 'APIProblem':
        assert isinstance(obj, dict)
        category = APIProblemCategory(obj.get("category"))
        code = from_str(obj.get("code"))
        detail = from_str(obj.get("detail"))
        instance = from_str(obj.get("instance"))
        retryable = from_bool(obj.get("retryable"))
        schema_version = SchemaVersion(obj.get("schemaVersion"))
        status = from_int(obj.get("status"))
        title = from_str(obj.get("title"))
        trace_id = from_str(obj.get("traceId"))
        type = from_str(obj.get("type"))
        retry_after_seconds = from_union([from_int, from_none], obj.get("retryAfterSeconds"))
        run_id = from_union([from_str, from_none], obj.get("runId"))
        return APIProblem(category, code, detail, instance, retryable, schema_version, status, title, trace_id, type, retry_after_seconds, run_id)

    def to_dict(self) -> dict:
        result: dict = {}
        result["category"] = to_enum(APIProblemCategory, self.category)
        result["code"] = from_str(self.code)
        result["detail"] = from_str(self.detail)
        result["instance"] = from_str(self.instance)
        result["retryable"] = from_bool(self.retryable)
        result["schemaVersion"] = to_enum(SchemaVersion, self.schema_version)
        result["status"] = from_int(self.status)
        result["title"] = from_str(self.title)
        result["traceId"] = from_str(self.trace_id)
        result["type"] = from_str(self.type)
        if self.retry_after_seconds is not None:
            result["retryAfterSeconds"] = from_union([from_int, from_none], self.retry_after_seconds)
        if self.run_id is not None:
            result["runId"] = from_union([from_str, from_none], self.run_id)
        return result


def rule_from_dict(s: Any) -> Rule:
    return Rule.from_dict(s)


def rule_to_dict(x: Rule) -> Any:
    return to_class(Rule, x)


def run_manifest_from_dict(s: Any) -> RunManifest:
    return RunManifest.from_dict(s)


def run_manifest_to_dict(x: RunManifest) -> Any:
    return to_class(RunManifest, x)


def finding_from_dict(s: Any) -> Finding:
    return Finding.from_dict(s)


def finding_to_dict(x: Finding) -> Any:
    return to_class(Finding, x)


def evidence_from_dict(s: Any) -> Evidence:
    return Evidence.from_dict(s)


def evidence_to_dict(x: Evidence) -> Any:
    return to_class(Evidence, x)


def report_from_dict(s: Any) -> Report:
    return Report.from_dict(s)


def report_to_dict(x: Report) -> Any:
    return to_class(Report, x)


def api_problem_from_dict(s: Any) -> APIProblem:
    return APIProblem.from_dict(s)


def api_problem_to_dict(x: APIProblem) -> Any:
    return to_class(APIProblem, x)
