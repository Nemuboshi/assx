package lsp

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"
	"strings"

	"assx/internal/analysis"
	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

type settings struct {
	Options analysis.Options
}

func parseSettings(raw jsontext.Value) (settings, error) {
	result := settings{Options: analysis.Options{Targets: lint.DefaultAnalysisTargets()}}
	if len(raw) == 0 || raw.Kind() == 'n' {
		return result, nil
	}
	var envelope struct {
		Assx *struct {
			Renderers []string `json:"renderers"`
			Rules     map[string]struct {
				Disabled bool          `json:"disabled"`
				Severity lint.Severity `json:"severity"`
			} `json:"rules"`
		} `json:"assx"`
	}
	if err := json.Unmarshal(raw, &envelope, json.RejectUnknownMembers(true)); err != nil {
		return settings{}, err
	}
	if envelope.Assx == nil {
		return result, nil
	}
	config := envelope.Assx
	if config.Renderers != nil {
		targets, err := rendererTargets(config.Renderers)
		if err != nil {
			return settings{}, err
		}
		result.Options.Targets = targets
	}
	for _, id := range slices.Sorted(maps.Keys(config.Rules)) {
		rule := config.Rules[id]
		result.Options.Rules = append(result.Options.Rules, lint.RuleOverride{Condition: id, Disabled: rule.Disabled, Severity: rule.Severity})
	}
	if err := result.Options.Rules.Validate(); err != nil {
		return settings{}, err
	}
	return result, nil
}

// rendererTargets reads one ordered list. The first name is primary.
func rendererTargets(names []string) (lint.AnalysisTargets, error) {
	if len(names) == 0 {
		return lint.AnalysisTargets{}, fmt.Errorf("renderers must name at least one renderer")
	}
	profiles := make([]renderer.Profile, 0, len(names))
	for _, name := range names {
		profile, err := configuredProfile(name)
		if err != nil {
			return lint.AnalysisTargets{}, err
		}
		if slices.Contains(profiles, profile) {
			continue
		}
		profiles = append(profiles, profile)
	}
	targets := lint.AnalysisTargets{Primary: profiles[0], Compatibility: slices.Clone(profiles[1:])}
	sorted := targets.Profiles()
	targets.Compatibility = nil
	for _, profile := range sorted {
		if profile != targets.Primary {
			targets.Compatibility = append(targets.Compatibility, profile)
		}
	}
	return targets, nil
}

func configuredProfile(name string) (renderer.Profile, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "libass":
		return renderer.Standard(renderer.Libass)
	case "xy-vsfilter":
		return renderer.Standard(renderer.XYVSFilter)
	case "vsfiltermod":
		return renderer.Standard(renderer.VSFilterMod)
	default:
		return renderer.Profile{}, fmt.Errorf("unknown renderer %q", name)
	}
}
