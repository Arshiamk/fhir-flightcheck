// To parse this data:
//
//   import { Convert, Rule, RunManifest, Finding, Evidence, Report, APIProblem } from "./contracts";
//
//   const rule = Convert.toRule(json);
//   const runManifest = Convert.toRunManifest(json);
//   const finding = Convert.toFinding(json);
//   const evidence = Convert.toEvidence(json);
//   const report = Convert.toReport(json);
//   const aPIProblem = Convert.toAPIProblem(json);
//
// These functions will throw an error if the JSON doesn't
// match the expected interface, even if the JSON is valid.

export interface Rule {
    behavior:               Behavior;
    capabilities:           Capability[];
    category:               RuleCategory;
    description:            string;
    deterministic:          boolean;
    evaluator:              string;
    id:                     string;
    remediation:            string;
    replacedBy?:            string;
    schemaVersion:          SchemaVersion;
    severity:               Severity;
    standardReferences?:    string[];
    supportedFhirVersions?: [FhirVersion, ...FhirVersion[]];
    timeoutSeconds:         number;
    title:                  string;
    version:                string;
}

export type Behavior = "passive" | "active-read" | "active-write";

export type Capability = "network" | "target-credentials" | "fixtures" | "model" | "write";

export type RuleCategory = "fhir" | "reliability" | "security" | "ai-safety";

export type SchemaVersion = "1.0.0";

export type Severity = "info" | "low" | "medium" | "high" | "critical";

export type FhirVersion = "4.0.1";

export interface RunManifest {
    createdAt:       Date;
    fixtureVersion?: string;
    modelVersions?:  { [key: string]: string };
    organizationId:  string;
    profile:         string;
    projectId:       string;
    ruleVersions:    { [key: string]: string };
    runId:           string;
    schemaVersion:   SchemaVersion;
    target:          Target;
}

export interface Target {
    allowPrivateNetwork?: boolean;
    baseUrl:              string;
    credentialRef:        string;
    fhirVersion:          FhirVersion;
    id:                   string;
}

export interface Finding {
    evidenceRefs:  string[];
    findingId:     string;
    observedAt:    Date;
    outcome:       Outcome;
    remediation:   string;
    ruleId:        string;
    ruleVersion:   string;
    runId:         string;
    schemaVersion: SchemaVersion;
    severity:      Severity;
    summary:       string;
    title:         string;
}

export type Outcome = "pass" | "fail" | "warning" | "not_applicable" | "inconclusive" | "platform_error";

export interface Evidence {
    createdAt:       Date;
    evidenceId:      string;
    mediaType:       string;
    redactionStatus: RedactionStatus;
    runId:           string;
    schemaVersion:   SchemaVersion;
    sha256:          string;
    sizeBytes:       number;
    sourceRuleId?:   string;
    storageUri:      string;
}

export type RedactionStatus = "not-required" | "redacted" | "rejected";

export interface Report {
    coverage:       Coverage;
    createdAt:      Date;
    decision:       Decision;
    findings:       ReportSchema[];
    manifestSha256: string;
    reportId:       string;
    runId:          string;
    schemaVersion:  SchemaVersion;
    signature?:     Signature;
}

export interface Coverage {
    completed: number;
    selected:  number;
}

export type Decision = "ready" | "conditional" | "not_ready" | "incomplete";

export interface ReportSchema {
    evidenceRefs:  string[];
    findingId:     string;
    observedAt:    Date;
    outcome:       Outcome;
    remediation:   string;
    ruleId:        string;
    ruleVersion:   string;
    runId:         string;
    schemaVersion: SchemaVersion;
    severity:      Severity;
    summary:       string;
    title:         string;
}

export interface Signature {
    algorithm: Algorithm;
    keyId:     string;
    value:     string;
}

export type Algorithm = "Ed25519";

export interface APIProblem {
    category:           APIProblemCategory;
    code:               string;
    detail:             string;
    instance:           string;
    retryable:          boolean;
    retryAfterSeconds?: number;
    runId?:             string;
    schemaVersion:      SchemaVersion;
    status:             number;
    title:              string;
    traceId:            string;
    type:               string;
    [property: string]: unknown;
}

export type APIProblemCategory = "configuration" | "target" | "transient_target" | "permanent_target" | "rule_defect" | "platform" | "authorization";

// Converts JSON strings to/from your types
// and asserts the results of JSON.parse at runtime
export class Convert {
    public static toRule(json: string): Rule {
        return cast(JSON.parse(json), r("Rule"));
    }

    public static ruleToJson(value: Rule): string {
        return JSON.stringify(uncast(value, r("Rule")), null, 2);
    }

    public static toRunManifest(json: string): RunManifest {
        return cast(JSON.parse(json), r("RunManifest"));
    }

    public static runManifestToJson(value: RunManifest): string {
        return JSON.stringify(uncast(value, r("RunManifest")), null, 2);
    }

    public static toFinding(json: string): Finding {
        return cast(JSON.parse(json), r("Finding"));
    }

    public static findingToJson(value: Finding): string {
        return JSON.stringify(uncast(value, r("Finding")), null, 2);
    }

    public static toEvidence(json: string): Evidence {
        return cast(JSON.parse(json), r("Evidence"));
    }

    public static evidenceToJson(value: Evidence): string {
        return JSON.stringify(uncast(value, r("Evidence")), null, 2);
    }

    public static toReport(json: string): Report {
        return cast(JSON.parse(json), r("Report"));
    }

    public static reportToJson(value: Report): string {
        return JSON.stringify(uncast(value, r("Report")), null, 2);
    }

    public static toAPIProblem(json: string): APIProblem {
        return cast(JSON.parse(json), r("APIProblem"));
    }

    public static aPIProblemToJson(value: APIProblem): string {
        return JSON.stringify(uncast(value, r("APIProblem")), null, 2);
    }
}

function invalidValue(typ: any, val: any, key: any, parent: any = ''): never {
    const prettyTyp = prettyTypeName(typ);
    const parentText = parent ? ` on ${parent}` : '';
    const keyText = key ? ` for key "${key}"` : '';
    throw Error(`Invalid value${keyText}${parentText}. Expected ${prettyTyp} but got ${JSON.stringify(val)}`);
}

function prettyTypeName(typ: any): string {
    if (Array.isArray(typ)) {
        if (typ.length === 2 && typ[0] === undefined) {
            return `an optional ${prettyTypeName(typ[1])}`;
        } else {
            return `one of [${typ.map(a => { return prettyTypeName(a); }).join(", ")}]`;
        }
    } else if (typeof typ === "object" && typ.literal !== undefined) {
        return typ.literal;
    } else {
        return typeof typ;
    }
}

function jsonToJSProps(typ: any): any {
    if (typ.jsonToJS === undefined) {
        const map: any = {};
        typ.props.forEach((p: any) => map[p.json] = { key: p.js, typ: p.typ });
        typ.jsonToJS = map;
    }
    return typ.jsonToJS;
}

function jsToJSONProps(typ: any): any {
    if (typ.jsToJSON === undefined) {
        const map: any = {};
        typ.props.forEach((p: any) => map[p.js] = { key: p.json, typ: p.typ });
        typ.jsToJSON = map;
    }
    return typ.jsToJSON;
}

function transform(val: any, typ: any, getProps: any, key: any = '', parent: any = ''): any {
    function transformPrimitive(typ: string, val: any): any {
        if (typeof typ === typeof val) return val;
        return invalidValue(typ, val, key, parent);
    }

    function transformUnion(typs: any[], val: any): any {
        // val must validate against one typ in typs
        const l = typs.length;
        for (let i = 0; i < l; i++) {
            const typ = typs[i];
            try {
                return transform(val, typ, getProps);
            } catch (_) {}
        }
        return invalidValue(typs, val, key, parent);
    }

    function transformEnum(cases: string[], val: any): any {
        if (cases.indexOf(val) !== -1) return val;
        return invalidValue(cases.map(a => { return l(a); }), val, key, parent);
    }

    function transformArray(typ: any, val: any): any {
        // val must be an array with no invalid elements
        if (!Array.isArray(val)) return invalidValue(l("array"), val, key, parent);
        return val.map(el => transform(el, typ, getProps));
    }

    function transformDate(val: any): any {
        if (val === null) {
            return null;
        }
        const d = new Date(val);
        if (isNaN(d.valueOf())) {
            return invalidValue(l("Date"), val, key, parent);
        }
        return d;
    }

    function transformObject(props: { [k: string]: any }, additional: any, val: any): any {
        if (val === null || typeof val !== "object" || Array.isArray(val)) {
            return invalidValue(l(ref || "object"), val, key, parent);
        }
        const result: any = {};
        Object.getOwnPropertyNames(props).forEach(key => {
            const prop = props[key];
            const v = Object.prototype.hasOwnProperty.call(val, key) ? val[key] : undefined;
            result[prop.key] = transform(v, prop.typ, getProps, key, ref);
        });
        Object.getOwnPropertyNames(val).forEach(key => {
            if (!Object.prototype.hasOwnProperty.call(props, key)) {
                result[key] = transform(val[key], additional, getProps, key, ref);
            }
        });
        return result;
    }

    if (typ === "any") return val;
    if (typ === null) {
        if (val === null) return val;
        return invalidValue(typ, val, key, parent);
    }
    if (typ === false) return invalidValue(typ, val, key, parent);
    let ref: any = undefined;
    while (typeof typ === "object" && typ.ref !== undefined) {
        ref = typ.ref;
        typ = typeMap[typ.ref];
    }
    if (Array.isArray(typ)) return transformEnum(typ, val);
    if (typeof typ === "object") {
        return typ.hasOwnProperty("unionMembers") ? transformUnion(typ.unionMembers, val)
            : typ.hasOwnProperty("arrayItems")    ? transformArray(typ.arrayItems, val)
            : typ.hasOwnProperty("props")         ? transformObject(getProps(typ), typ.additional, val)
            : invalidValue(typ, val, key, parent);
    }
    // Numbers can be parsed by Date but shouldn't be.
    if (typ === Date && typeof val !== "number") return transformDate(val);
    return transformPrimitive(typ, val);
}

function cast<T>(val: any, typ: any): T {
    return transform(val, typ, jsonToJSProps);
}

function uncast<T>(val: T, typ: any): any {
    return transform(val, typ, jsToJSONProps);
}

function l(typ: any) {
    return { literal: typ };
}

function a(typ: any) {
    return { arrayItems: typ };
}

function u(...typs: any[]) {
    return { unionMembers: typs };
}

function o(props: any[], additional: any) {
    return { props, additional };
}

function m(additional: any) {
    return { props: [], additional };
}

function r(name: string) {
    return { ref: name };
}

const typeMap: any = {
    "Rule": o([
        { json: "behavior", js: "behavior", typ: r("Behavior") },
        { json: "capabilities", js: "capabilities", typ: a(r("Capability")) },
        { json: "category", js: "category", typ: r("RuleCategory") },
        { json: "description", js: "description", typ: "" },
        { json: "deterministic", js: "deterministic", typ: true },
        { json: "evaluator", js: "evaluator", typ: "" },
        { json: "id", js: "id", typ: "" },
        { json: "remediation", js: "remediation", typ: "" },
        { json: "replacedBy", js: "replacedBy", typ: u(undefined, "") },
        { json: "schemaVersion", js: "schemaVersion", typ: r("SchemaVersion") },
        { json: "severity", js: "severity", typ: r("Severity") },
        { json: "standardReferences", js: "standardReferences", typ: u(undefined, a("")) },
        { json: "supportedFhirVersions", js: "supportedFhirVersions", typ: u(undefined, a(r("FhirVersion"))) },
        { json: "timeoutSeconds", js: "timeoutSeconds", typ: 0 },
        { json: "title", js: "title", typ: "" },
        { json: "version", js: "version", typ: "" },
    ], false),
    "RunManifest": o([
        { json: "createdAt", js: "createdAt", typ: Date },
        { json: "fixtureVersion", js: "fixtureVersion", typ: u(undefined, "") },
        { json: "modelVersions", js: "modelVersions", typ: u(undefined, m("")) },
        { json: "organizationId", js: "organizationId", typ: "" },
        { json: "profile", js: "profile", typ: "" },
        { json: "projectId", js: "projectId", typ: "" },
        { json: "ruleVersions", js: "ruleVersions", typ: m("") },
        { json: "runId", js: "runId", typ: "" },
        { json: "schemaVersion", js: "schemaVersion", typ: r("SchemaVersion") },
        { json: "target", js: "target", typ: r("Target") },
    ], false),
    "Target": o([
        { json: "allowPrivateNetwork", js: "allowPrivateNetwork", typ: u(undefined, true) },
        { json: "baseUrl", js: "baseUrl", typ: "" },
        { json: "credentialRef", js: "credentialRef", typ: "" },
        { json: "fhirVersion", js: "fhirVersion", typ: r("FhirVersion") },
        { json: "id", js: "id", typ: "" },
    ], false),
    "Finding": o([
        { json: "evidenceRefs", js: "evidenceRefs", typ: a("") },
        { json: "findingId", js: "findingId", typ: "" },
        { json: "observedAt", js: "observedAt", typ: Date },
        { json: "outcome", js: "outcome", typ: r("Outcome") },
        { json: "remediation", js: "remediation", typ: "" },
        { json: "ruleId", js: "ruleId", typ: "" },
        { json: "ruleVersion", js: "ruleVersion", typ: "" },
        { json: "runId", js: "runId", typ: "" },
        { json: "schemaVersion", js: "schemaVersion", typ: r("SchemaVersion") },
        { json: "severity", js: "severity", typ: r("Severity") },
        { json: "summary", js: "summary", typ: "" },
        { json: "title", js: "title", typ: "" },
    ], false),
    "Evidence": o([
        { json: "createdAt", js: "createdAt", typ: Date },
        { json: "evidenceId", js: "evidenceId", typ: "" },
        { json: "mediaType", js: "mediaType", typ: "" },
        { json: "redactionStatus", js: "redactionStatus", typ: r("RedactionStatus") },
        { json: "runId", js: "runId", typ: "" },
        { json: "schemaVersion", js: "schemaVersion", typ: r("SchemaVersion") },
        { json: "sha256", js: "sha256", typ: "" },
        { json: "sizeBytes", js: "sizeBytes", typ: 0 },
        { json: "sourceRuleId", js: "sourceRuleId", typ: u(undefined, "") },
        { json: "storageUri", js: "storageUri", typ: "" },
    ], false),
    "Report": o([
        { json: "coverage", js: "coverage", typ: r("Coverage") },
        { json: "createdAt", js: "createdAt", typ: Date },
        { json: "decision", js: "decision", typ: r("Decision") },
        { json: "findings", js: "findings", typ: a(r("ReportSchema")) },
        { json: "manifestSha256", js: "manifestSha256", typ: "" },
        { json: "reportId", js: "reportId", typ: "" },
        { json: "runId", js: "runId", typ: "" },
        { json: "schemaVersion", js: "schemaVersion", typ: r("SchemaVersion") },
        { json: "signature", js: "signature", typ: u(undefined, r("Signature")) },
    ], false),
    "Coverage": o([
        { json: "completed", js: "completed", typ: 0 },
        { json: "selected", js: "selected", typ: 0 },
    ], false),
    "ReportSchema": o([
        { json: "evidenceRefs", js: "evidenceRefs", typ: a("") },
        { json: "findingId", js: "findingId", typ: "" },
        { json: "observedAt", js: "observedAt", typ: Date },
        { json: "outcome", js: "outcome", typ: r("Outcome") },
        { json: "remediation", js: "remediation", typ: "" },
        { json: "ruleId", js: "ruleId", typ: "" },
        { json: "ruleVersion", js: "ruleVersion", typ: "" },
        { json: "runId", js: "runId", typ: "" },
        { json: "schemaVersion", js: "schemaVersion", typ: r("SchemaVersion") },
        { json: "severity", js: "severity", typ: r("Severity") },
        { json: "summary", js: "summary", typ: "" },
        { json: "title", js: "title", typ: "" },
    ], false),
    "Signature": o([
        { json: "algorithm", js: "algorithm", typ: r("Algorithm") },
        { json: "keyId", js: "keyId", typ: "" },
        { json: "value", js: "value", typ: "" },
    ], false),
    "APIProblem": o([
        { json: "category", js: "category", typ: r("APIProblemCategory") },
        { json: "code", js: "code", typ: "" },
        { json: "detail", js: "detail", typ: "" },
        { json: "instance", js: "instance", typ: "" },
        { json: "retryable", js: "retryable", typ: true },
        { json: "retryAfterSeconds", js: "retryAfterSeconds", typ: u(undefined, 0) },
        { json: "runId", js: "runId", typ: u(undefined, "") },
        { json: "schemaVersion", js: "schemaVersion", typ: r("SchemaVersion") },
        { json: "status", js: "status", typ: 0 },
        { json: "title", js: "title", typ: "" },
        { json: "traceId", js: "traceId", typ: "" },
        { json: "type", js: "type", typ: "" },
    ], "any"),
    "Behavior": [
        "active-read",
        "active-write",
        "passive",
    ],
    "Capability": [
        "fixtures",
        "model",
        "network",
        "target-credentials",
        "write",
    ],
    "RuleCategory": [
        "ai-safety",
        "fhir",
        "reliability",
        "security",
    ],
    "SchemaVersion": [
        "1.0.0",
    ],
    "Severity": [
        "critical",
        "high",
        "info",
        "low",
        "medium",
    ],
    "FhirVersion": [
        "4.0.1",
    ],
    "Outcome": [
        "fail",
        "inconclusive",
        "not_applicable",
        "pass",
        "platform_error",
        "warning",
    ],
    "RedactionStatus": [
        "not-required",
        "redacted",
        "rejected",
    ],
    "Decision": [
        "conditional",
        "incomplete",
        "not_ready",
        "ready",
    ],
    "Algorithm": [
        "Ed25519",
    ],
    "APIProblemCategory": [
        "authorization",
        "configuration",
        "permanent_target",
        "platform",
        "rule_defect",
        "target",
        "transient_target",
    ],
};
