from __future__ import annotations

import json
from collections.abc import Callable, Iterable
from pathlib import Path
from typing import Any

from .models import Capability, EvaluationResult, ProbeSnapshot, Rule

RuleEvaluator = Callable[[ProbeSnapshot], EvaluationResult]


class RegistryError(ValueError):
    pass


class RuleRegistry:
    """Closed registry: catalogs can select known evaluators, never import code."""

    def __init__(self, evaluators: dict[str, RuleEvaluator]) -> None:
        self._evaluators = evaluators.copy()
        self._rules: dict[str, Rule] = {}

    def register(self, raw: dict[str, Any]) -> Rule:
        rule = Rule.model_validate(raw)
        if rule.evaluator not in self._evaluators:
            raise RegistryError(f"unknown evaluator: {rule.evaluator}")
        if rule.id in self._rules:
            raise RegistryError(f"duplicate rule id: {rule.id}")
        if Capability.WRITE in rule.capabilities and rule.behavior != "active-write":
            raise RegistryError("write capability is only valid for active-write rules")
        self._rules[rule.id] = rule
        return rule

    def load_file(self, path: Path) -> list[Rule]:
        payload = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(payload, dict) or not isinstance(payload.get("rules"), list):
            raise RegistryError(f"{path} is not a rule pack")
        return [self.register(raw) for raw in payload["rules"]]

    def load_directory(self, path: Path) -> list[Rule]:
        loaded: list[Rule] = []
        for candidate in sorted(path.glob("*.json")):
            loaded.extend(self.load_file(candidate))
        return loaded

    def select(
        self, rule_ids: Iterable[str], granted: frozenset[Capability]
    ) -> list[tuple[Rule, RuleEvaluator]]:
        selected: list[tuple[Rule, RuleEvaluator]] = []
        for rule_id in rule_ids:
            rule = self._rules.get(rule_id)
            if rule is None:
                raise RegistryError(f"unknown rule id: {rule_id}")
            missing = rule.capabilities - granted
            if missing:
                names = ", ".join(sorted(missing))
                raise RegistryError(f"{rule_id} requires ungranted capabilities: {names}")
            selected.append((rule, self._evaluators[rule.evaluator]))
        return selected

    @property
    def rules(self) -> tuple[Rule, ...]:
        return tuple(self._rules[key] for key in sorted(self._rules))
