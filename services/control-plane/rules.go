package controlplane

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type RuleCatalog struct {
	rules []Rule
}

func LoadRuleCatalog(directory string) (*RuleCatalog, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read rule-pack directory: %w", err)
	}
	var rules []Rule
	seen := make(map[string]struct{})
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read rule pack %s: %w", entry.Name(), err)
		}
		var pack struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Rules   []Rule `json:"rules"`
		}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&pack); err != nil {
			return nil, fmt.Errorf("decode rule pack %s: %w", entry.Name(), err)
		}
		for _, rule := range pack.Rules {
			if err := validateRule(rule); err != nil {
				return nil, fmt.Errorf("%s: %w", entry.Name(), err)
			}
			if _, ok := seen[rule.ID]; ok {
				return nil, fmt.Errorf("duplicate rule %q", rule.ID)
			}
			seen[rule.ID] = struct{}{}
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil, errors.New("no rules loaded")
	}
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return &RuleCatalog{rules: rules}, nil
}

func validateRule(rule Rule) error {
	if rule.SchemaVersion != SchemaVersion || rule.ID == "" || rule.Version == "" || rule.Evaluator == "" {
		return fmt.Errorf("rule %q is missing required contract fields", rule.ID)
	}
	if rule.Severity != "info" && rule.Severity != "low" && rule.Severity != "medium" && rule.Severity != "high" && rule.Severity != "critical" {
		return fmt.Errorf("rule %q has invalid severity", rule.ID)
	}
	for _, capability := range rule.Capabilities {
		if capability == "target-credentials" || capability == "write" {
			return fmt.Errorf("rule %q requests a capability excluded from the startup profile", rule.ID)
		}
	}
	return nil
}

func (c *RuleCatalog) Rules() []Rule {
	return append([]Rule(nil), c.rules...)
}

func (c *RuleCatalog) Versions() map[string]string {
	versions := make(map[string]string, len(c.rules))
	for _, rule := range c.rules {
		versions[rule.ID] = rule.Version
	}
	return versions
}

func (c *RuleCatalog) Capabilities() []string {
	set := make(map[string]struct{})
	for _, rule := range c.rules {
		for _, capability := range rule.Capabilities {
			set[capability] = struct{}{}
		}
	}
	values := make([]string, 0, len(set))
	for capability := range set {
		values = append(values, capability)
	}
	sort.Strings(values)
	return values
}

func (c *RuleCatalog) ByID() map[string]Rule {
	values := make(map[string]Rule, len(c.rules))
	for _, rule := range c.rules {
		values[rule.ID] = rule
	}
	return values
}
