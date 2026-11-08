package main

import (
	"flag"
	"fmt"
	"strings"

	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

func parseAnalysisTargets(flags *flag.FlagSet, primaryName, compatNames string) (lint.AnalysisTargets, error) {
	kind, err := parseRenderer(primaryName)
	if err != nil {
		return lint.AnalysisTargets{}, err
	}

	explicitRenderer, explicitCompat := false, false
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "renderer":
			explicitRenderer = true
		case "compat":
			explicitCompat = true
		}
	})
	if compatNames == "" && !explicitRenderer && !explicitCompat {
		compatNames = "xy-vsfilter"
	}
	var compatKinds []renderer.Kind
	if compatNames != "" {
		for _, name := range strings.Split(compatNames, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				return lint.AnalysisTargets{}, fmt.Errorf("invalid --compat list: renderer names cannot be empty")
			}
			target, err := parseRenderer(name)
			if err != nil {
				return lint.AnalysisTargets{}, err
			}
			compatKinds = append(compatKinds, target)
		}
	}
	primary, err := renderer.New(kind)
	if err != nil {
		return lint.AnalysisTargets{}, err
	}
	targets := lint.AnalysisTargets{Primary: primary}
	for _, target := range compatKinds {
		profile, err := renderer.Standard(target)
		if err != nil {
			return lint.AnalysisTargets{}, err
		}
		targets.Compatibility = append(targets.Compatibility, profile)
	}
	// Normalize compatibility order and remove the primary from the extra set.
	profiles := targets.Profiles()
	targets.Compatibility = nil
	for _, profile := range profiles {
		if profile != primary {
			targets.Compatibility = append(targets.Compatibility, profile)
		}
	}
	return targets, nil
}

func parseRenderer(name string) (renderer.Kind, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "libass":
		return renderer.Libass, nil
	case "xy-vsfilter":
		return renderer.XYVSFilter, nil
	case "vsfiltermod":
		return renderer.VSFilterMod, nil
	default:
		return 0, fmt.Errorf("unknown renderer %q: use libass, xy-vsfilter, or vsfiltermod", name)
	}
}
