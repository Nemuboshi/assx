package main

import (
	"flag"
	"fmt"
	"strings"

	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

func parseAnalysisTargets(flags *flag.FlagSet, primaryName, compatNames, modName, luaName string) (lint.AnalysisTargets, error) {
	kind, err := parseRenderer(primaryName)
	if err != nil {
		return lint.AnalysisTargets{}, err
	}
	mod, err := parseFeature(modName)
	if err != nil {
		return lint.AnalysisTargets{}, fmt.Errorf("invalid --mod-build %q: use unknown, enabled, or disabled", modName)
	}
	lua, err := parseFeature(luaName)
	if err != nil {
		return lint.AnalysisTargets{}, fmt.Errorf("invalid --lua-build %q: use unknown, enabled, or disabled", luaName)
	}

	explicitRenderer, explicitCompat, explicitBuild := false, false, false
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "renderer":
			explicitRenderer = true
		case "compat":
			explicitCompat = true
		case "mod-build", "lua-build":
			explicitBuild = true
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
	includesMod := kind == renderer.VSFilterMod
	for _, target := range compatKinds {
		includesMod = includesMod || target == renderer.VSFilterMod
	}
	if !includesMod && explicitBuild {
		return lint.AnalysisTargets{}, fmt.Errorf("--mod-build and --lua-build require --renderer=vsfiltermod or --compat including vsfiltermod")
	}

	build := renderer.Build{}
	if kind == renderer.VSFilterMod {
		build = renderer.Build{Mod: mod, Lua: lua}
	}
	primary, err := renderer.New(kind, build)
	if err != nil {
		return lint.AnalysisTargets{}, err
	}
	targets := lint.AnalysisTargets{Primary: primary}
	for _, target := range compatKinds {
		if target == renderer.VSFilterMod {
			profile, err := renderer.New(target, renderer.Build{Mod: mod, Lua: lua})
			if err != nil {
				return lint.AnalysisTargets{}, err
			}
			targets.Compatibility = append(targets.Compatibility, profile)
			continue
		}
		profile, _ := renderer.Standard(target)
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

func parseFeature(name string) (renderer.Feature, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "unknown":
		return renderer.FeatureUnknown, nil
	case "enabled":
		return renderer.FeatureEnabled, nil
	case "disabled":
		return renderer.FeatureDisabled, nil
	default:
		return renderer.FeatureUnknown, fmt.Errorf("invalid build capability %q", name)
	}
}
