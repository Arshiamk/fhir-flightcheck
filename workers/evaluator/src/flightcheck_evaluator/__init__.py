"""Deterministic, capability-restricted FHIR Flightcheck evaluator."""

from .evaluator import Evaluator, HttpProber
from .models import Finding, ProbeSnapshot, Rule, RunManifest
from .registry import RuleRegistry

__all__ = [
    "Evaluator",
    "Finding",
    "HttpProber",
    "ProbeSnapshot",
    "Rule",
    "RuleRegistry",
    "RunManifest",
]


def hello() -> str:
    return "Hello from flightcheck-evaluator!"
