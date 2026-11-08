package lint

import "fmt"

// RuleOverride changes one rule for one analysis session. An empty Severity
// keeps the rule default.
type RuleOverride struct {
	Condition string
	Disabled  bool
	Severity  Severity
}

// RulePolicy lists per-condition overrides. Conditions must be known and unique, and a
// severity must be valid. Nil or empty keeps the default catalog. The policy
// controls findings and authorization. It never disables semantic state
// transitions.
type RulePolicy []RuleOverride

// Validate rejects an override with no condition, a repeated condition, an unknown condition,
// or an invalid severity. Nil and empty policies are valid.
func (policy RulePolicy) Validate() error { return policy.validate() }

func (policy RulePolicy) validate() error {
	seen := make(map[string]bool, len(policy))
	for _, override := range policy {
		if override.Condition == "" {
			return fmt.Errorf("rule policy override has no condition")
		}
		if seen[override.Condition] {
			return fmt.Errorf("rule policy repeats condition %s", override.Condition)
		}
		seen[override.Condition] = true
		if _, known := rules[override.Condition]; !known {
			return fmt.Errorf("rule policy names unknown condition %s", override.Condition)
		}
		switch override.Severity {
		case "", Error, Warning, Suggestion:
		default:
			return fmt.Errorf("rule policy for %s has invalid severity %q", override.Condition, override.Severity)
		}
	}
	return nil
}

func (policy RulePolicy) disabled(id string) bool {
	for _, override := range policy {
		if override.Condition == id {
			return override.Disabled
		}
	}
	return false
}

// apply drops disabled rules and rewrites severities after discovery, in
// report order. It never mutates the global condition catalog.
func (policy RulePolicy) apply(findings []Diagnostic) []Diagnostic {
	if len(policy) == 0 {
		return findings
	}
	overrides := make(map[string]RuleOverride, len(policy))
	for _, override := range policy {
		overrides[override.Condition] = override
	}
	out := make([]Diagnostic, 0, len(findings))
	for _, diagnostic := range findings {
		override, overridden := overrides[diagnostic.Condition]
		if overridden && override.Disabled {
			continue
		}
		if overridden && override.Severity != "" {
			diagnostic.Severity = override.Severity
		}
		out = append(out, diagnostic)
	}
	return out
}
