package main

import (
	"fmt"
	"strings"

	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

func parseAnalysisTargets(rendererNames string) (lint.AnalysisTargets, error) {
	kinds := make([]renderer.Kind, 0, 3)
	seen := make(map[renderer.Kind]bool, 3)
	for _, name := range strings.Split(rendererNames, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			return lint.AnalysisTargets{}, fmt.Errorf("invalid --renderer list: renderer names cannot be empty")
		}
		kind, err := parseRenderer(name)
		if err != nil {
			return lint.AnalysisTargets{}, err
		}
		if seen[kind] {
			continue
		}
		seen[kind] = true
		kinds = append(kinds, kind)
	}
	primary, err := renderer.New(kinds[0])
	if err != nil {
		return lint.AnalysisTargets{}, err
	}
	targets := lint.AnalysisTargets{Primary: primary}
	for _, kind := range kinds[1:] {
		profile, err := renderer.Standard(kind)
		if err != nil {
			return lint.AnalysisTargets{}, err
		}
		targets.Compatibility = append(targets.Compatibility, profile)
	}
	// Sort the comparison set and drop the primary from it.
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
