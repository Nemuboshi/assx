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
		return cmp.Or(cmp.Compare(a.Kind(), b.Kind()), cmp.Compare(a.Version(), b.Version()))
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

// ProfileScope identifies the pinned renderer and revision.
func ProfileScope(profile renderer.Profile) string {
	return profile.Kind().String() + "@" + profile.Version()
}
