package renderer

import (
	"testing"

	"assx/internal/ass/spec"
)

// legacySpec is the selection CommandSpec replaced: the global tables only.
func legacySpec(profile Profile, name string) (spec.TagSpec, bool) {
	tagSpec, known := spec.TagSpecs[name]
	if profile.Kind() == VSFilterMod {
		if mod, ok := spec.VSFilterModTagSpecs[name]; ok {
			tagSpec, known = mod, true
		}
	}
	return tagSpec, known
}

func TestCommandSpecMatchesFallback(t *testing.T) {
	profiles := []Profile{{kind: Libass}, {kind: XYVSFilter}, {kind: VSFilterMod}}
	names := make(map[string]bool)
	for name := range spec.TagSpecs {
		names[name] = true
	}
	for name := range spec.VSFilterModTagSpecs {
		names[name] = true
	}
	for name := range names {
		for _, profile := range profiles {
			want, wantOK := legacySpec(profile, name)
			for count := 0; count <= 8; count++ {
				for _, form := range []Form{Bare, Paren} {
					got, ok := profile.CommandSpec(name, count, form)
					if ok != wantOK || got.Value != want.Value || got.Min != want.Min || got.Max != want.Max {
						t.Errorf("%s/%s count=%d form=%v: got (%v,%d,%d,%v), want (%v,%d,%d,%v)",
							profile.Kind(), name, count, form,
							got.Value, got.Min, got.Max, ok,
							want.Value, want.Min, want.Max, wantOK)
					}
				}
			}
		}
	}
}

// One tag can carry a different value range per renderer. The matrix row
// scopes the override; the fallback table does not leak into the other scope.
func TestCommandSpecUsesRendererScopedParams(t *testing.T) {
	original := matrixTags["q"]
	defer func() { matrixTags["q"] = original; rebuildCommandSpecs() }()
	matrixTags["q"] = tagMeta{signatures: []generatedSignature{
		{count: 1, form: Both, scope: bit(Libass), citation: "libass",
			params: []generatedParam{{index: 1, scope: bit(Libass), value: "integer", min: 0, max: 100, hasRange: true}}},
		{count: 1, form: Both, scope: bit(XYVSFilter), citation: "xy",
			params: []generatedParam{{index: 1, scope: bit(XYVSFilter), value: "integer", min: 0, max: 60, hasRange: true}}},
	}}
	rebuildCommandSpecs()
	libass := Profile{kind: Libass}
	xy := Profile{kind: XYVSFilter}
	mod := Profile{kind: VSFilterMod}
	if got, _ := libass.CommandSpec("q", 1, Both); got.Min != 0 || got.Max != 100 {
		t.Fatalf("libass range = %d..%d", got.Min, got.Max)
	}
	if got, _ := xy.CommandSpec("q", 1, Both); got.Min != 0 || got.Max != 60 {
		t.Fatalf("xy range = %d..%d", got.Min, got.Max)
	}
	// Mod has no matrix override for this tag, so the fallback range applies.
	if got, _ := mod.CommandSpec("q", 1, Both); got.Min != 0 || got.Max != 3 {
		t.Fatalf("mod fallback range = %d..%d", got.Min, got.Max)
	}
}

// Resolve must publish the selected spec, so lint sees the profile's range.
func TestResolvePolicyUsesRendererRange(t *testing.T) {
	original := matrixTags["q"]
	defer func() { matrixTags["q"] = original; rebuildCommandSpecs() }()
	matrixTags["q"] = tagMeta{signatures: []generatedSignature{
		{count: 1, form: Both, scope: bit(Libass),
			params: []generatedParam{{index: 1, scope: bit(Libass), value: "integer", min: 0, max: 100, hasRange: true}}},
		{count: 1, form: Both, scope: bit(XYVSFilter),
			params: []generatedParam{{index: 1, scope: bit(XYVSFilter), value: "integer", min: 0, max: 60, hasRange: true}}},
	}}
	rebuildCommandSpecs()
	expr, source := first(t, `\q61`)
	for _, test := range []struct {
		kind Kind
		max  int
	}{{Libass, 100}, {XYVSFilter, 60}} {
		profile, err := Standard(test.kind)
		if err != nil {
			t.Fatal(err)
		}
		result := profile.Resolve(expr, source)
		if !result.HasPolicy || result.Policy.Value != spec.IntegerValue || result.Policy.Max != test.max {
			t.Errorf("%s policy = %#v, want integer max %d", test.kind, result.Policy, test.max)
		}
	}
}
