package lint

import (
	"cmp"
	"slices"

	"assx/internal/ass/renderer"
)

// AnalysisTargets selects the renderer used for validity and the profiles
// included in compatibility and SafeFix checks.
type AnalysisTargets struct {
	Primary       renderer.Profile
	Compatibility []renderer.Profile
}

// Profiles returns a deterministic, duplicate-free proof target set.
func (targets AnalysisTargets) Profiles() []renderer.Profile {
	profiles := append([]renderer.Profile{targets.Primary}, targets.Compatibility...)
	slices.SortFunc(profiles, func(a, b renderer.Profile) int {
		return cmp.Or(cmp.Compare(a.Kind(), b.Kind()), cmp.Compare(a.Version(), b.Version()), cmp.Compare(a.Build().Mod, b.Build().Mod), cmp.Compare(a.Build().Lua, b.Build().Lua))
	})
	return slices.CompactFunc(profiles, func(a, b renderer.Profile) bool {
		return a == b
	})
}

// DefaultAnalysisTargets uses libass validity with libass and xy-VSFilter proof scope.
func DefaultAnalysisTargets() AnalysisTargets {
	libass, _ := renderer.Standard(renderer.Libass)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	return AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{xy}}
}

// ProfileScope identifies the pinned renderer and optional build capabilities.
func ProfileScope(profile renderer.Profile) string {
	if profile.Kind() != renderer.VSFilterMod {
		return profile.Kind().String() + "@" + profile.Version()
	}
	return profile.Kind().String() + "@" + profile.Version() + " (_VSMOD=" + featureName(profile.Build().Mod) + ", _LUA=" + featureName(profile.Build().Lua) + ")"
}
