package lsp

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"maps"
	"slices"

	"assx/internal/analysis"
	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

type settings struct {
	Options  analysis.Options
	ForceASS bool
}

func parseSettings(raw jsontext.Value) (settings, error) {
	result := settings{Options: analysis.Options{Targets: lint.DefaultAnalysisTargets()}}
	if len(raw) == 0 || raw.Kind() == 'n' {
		return result, nil
	}
	var envelope struct {
		Assx *struct {
			PrimaryRenderer        string   `json:"primaryRenderer"`
			CompatibilityRenderers []string `json:"compatibilityRenderers"`
			Rules                  map[string]struct {
				Disabled bool          `json:"disabled"`
				Severity lint.Severity `json:"severity"`
			} `json:"rules"`
			ForceASS bool `json:"forceASS"`
		} `json:"assx"`
	}
	if err := json.Unmarshal(raw, &envelope, json.RejectUnknownMembers(true)); err != nil {
		return settings{}, err
	}
	if envelope.Assx == nil {
		return result, nil
	}
	config := envelope.Assx
	if config.PrimaryRenderer != "" {
		profile, err := configuredProfile(config.PrimaryRenderer)
		result.Options.Targets.Primary = profile
		if err != nil {
			return settings{}, err
		}
		result.Options.Targets.Compatibility = []renderer.Profile{}
	}
	if config.CompatibilityRenderers != nil {
		result.Options.Targets.Compatibility = make([]renderer.Profile, 0, len(config.CompatibilityRenderers))
		for _, name := range config.CompatibilityRenderers {
			profile, err := configuredProfile(name)
			if err != nil {
				return settings{}, err
			}
			if profile == result.Options.Targets.Primary || slices.Contains(result.Options.Targets.Compatibility, profile) {
				return settings{}, fmt.Errorf("duplicate renderer %q", name)
			}
			result.Options.Targets.Compatibility = append(result.Options.Targets.Compatibility, profile)
		}
	}
	for _, id := range slices.Sorted(maps.Keys(config.Rules)) {
		rule := config.Rules[id]
		result.Options.Rules = append(result.Options.Rules, lint.RuleOverride{ID: id, Disabled: rule.Disabled, Severity: rule.Severity})
	}
	if err := result.Options.Rules.Validate(); err != nil {
		return settings{}, err
	}
	result.ForceASS = config.ForceASS
	return result, nil
}

func configuredProfile(name string) (renderer.Profile, error) {
	switch name {
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
