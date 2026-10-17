package lint

import "fmt"

// RuleOverride changes one rule for one analysis session. An empty Severity
// keeps the rule default.
type RuleOverride struct {
	ID       string
	Disabled bool
	Severity Severity
}

// RulePolicy lists per-rule overrides. IDs must be known and unique, and a
// severity must be valid. Nil or empty keeps the default catalog. The policy
// controls findings and authorization. It never disables semantic state
// transitions.
type RulePolicy []RuleOverride

func (policy RulePolicy) validate() error {
	seen := make(map[string]bool, len(policy))
	for _, override := range policy {
		if override.ID == "" {
			return fmt.Errorf("rule policy override has no ID")
		}
		if seen[override.ID] {
			return fmt.Errorf("rule policy repeats ID %s", override.ID)
		}
		seen[override.ID] = true
		if _, known := Rules[override.ID]; !known {
			return fmt.Errorf("rule policy names unknown rule %s", override.ID)
		}
		switch override.Severity {
		case "", Error, Warning, Suggestion:
		default:
			return fmt.Errorf("rule policy for %s has invalid severity %q", override.ID, override.Severity)
		}
	}
	return nil
}

func (policy RulePolicy) disabled(id string) bool {
	for _, override := range policy {
		if override.ID == id {
			return override.Disabled
		}
	}
	return false
}

// apply drops disabled rules and rewrites severities after discovery, in
// report order. It never mutates the global rule catalog.
func (policy RulePolicy) apply(findings []Diagnostic) []Diagnostic {
	if len(policy) == 0 {
		return findings
	}
	overrides := make(map[string]RuleOverride, len(policy))
	for _, override := range policy {
		overrides[override.ID] = override
	}
	out := make([]Diagnostic, 0, len(findings))
	for _, diagnostic := range findings {
		override, overridden := overrides[diagnostic.ID]
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
