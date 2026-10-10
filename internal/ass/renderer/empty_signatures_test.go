package renderer

import (
	"strings"
	"testing"
)

// Each of these names has a pinned renderer path that accepts the empty bare
// form. Keep this independent list explicit: deleting XML evidence must not
// silently turn an accepted reset/default into SignatureRejected.
//
// Some entries have only inferred evidence on one renderer; those must not be
// promoted to source-verified merely because another renderer accepts them.
var auditedBareEmptyTags = strings.Fields(
	"b i u s q fe fn fs fsc fscx fscy fsp bord xbord ybord shad xshad yshad " +
		"blur be frx fry frz fr fax fay c 1c 2c 3c 4c 1a 2a 3a 4a alpha an a " +
		"r k K kf ko kt p pbo blend frs fsvp",
)

func TestPinnedBareEmptyFormsHaveExplicitEvidence(t *testing.T) {
	p := profiles(t)
	buildDisabled, err := New(VSFilterMod, Build{Mod: FeatureDisabled})
	if err != nil {
		t.Fatal(err)
	}
	buildUnknown, err := Standard(VSFilterMod)
	if err != nil {
		t.Fatal(err)
	}
	choices := []Profile{p[0], p[1], p[2], buildDisabled, buildUnknown}

	for _, name := range auditedBareEmptyTags {
		t.Run(name, func(t *testing.T) {
			meta, exists := matrixTags[name]
			if !exists {
				t.Fatalf("audited command %q missing from evidence matrix", name)
			}
			hasZero := false
			for _, sig := range meta.signatures {
				if sig.count == 0 && sig.form == Bare {
					hasZero = true
					break
				}
			}
			if !hasZero {
				t.Fatalf("%s has no explicit bare zero-argument signature", name)
			}
			expr, source := first(t, "\\"+name)
			matchedProfiles := 0
			for _, profile := range choices {
				got := profile.Resolve(expr, source)
				if got.Status != Matched || got.Name != name {
					// Alternate renderer prefix selection or compile-time gate.
					continue
				}
				matchedProfiles++
				var want SignatureStatus
				found := false
				for _, sig := range profile.Signatures(name) {
					if sig.Count != 0 || sig.Form != Bare ||
						sig.Availability != SignatureAvailable {
						continue
					}
					found = true
					want = sig.Evidence
				}
				if !found {
					t.Errorf("%s mod=%v name=%q has no build-applicable bare zero-argument signature",
						profile.Kind(), profile.Build().Mod, name)
					continue
				}
				if got.Signature != want || got.Citation == "" {
					t.Errorf("%s mod=%v bare %q: signature=%v, citation=%q, want=%v",
						profile.Kind(), profile.Build().Mod, name, got.Signature, got.Citation, want)
				}
			}
			if matchedProfiles == 0 {
				t.Fatalf("%s was not selected by any pinned renderer profile", name)
			}
		})
	}
}

func TestTrulyRequiredBareArgumentsStillReject(t *testing.T) {
	expr, source := first(t, "\\pos")
	for _, profile := range profiles(t) {
		got := profile.Resolve(expr, source)
		if got.Status != Matched || got.Signature != SignatureRejected {
			t.Errorf("%s: bare position must remain rejected: %+v", profile.Kind(), got)
		}
	}
	for _, mod := range []struct {
		capability Feature
		expected   SignatureStatus
	}{
		{FeatureUnknown, SignatureUnknown},
		{FeatureDisabled, SignatureRejected},
		{FeatureEnabled, SignatureRejected},
	} {
		profile, err := New(VSFilterMod, Build{Mod: mod.capability})
		if err != nil {
			t.Fatal(err)
		}
		got := profile.Resolve(expr, source)
		if got.Signature != mod.expected {
			t.Errorf("VSFilterMod feature=%v: bare position signature=%v want=%v",
				mod.capability, got.Signature, mod.expected)
		}
	}
}

// The source-backed scalar handlers can consume the first value while ignoring
// surplus parenthesized text. Declared known forms alone cannot prove rejection.
func TestNonExhaustiveSignatureSetsCannotReject(t *testing.T) {
	for _, raw := range []string{"\\fs(18,unexpected)", "\\bord(2,3)", "\\fn(Arial,Extra)", "\\clip"} {
		expr, source := first(t, raw)
		for _, profile := range profiles(t) {
			got := profile.Resolve(expr, source)
			if got.Signature == SignatureRejected {
				t.Errorf("%s: non-exhaustive %q was rejected as a proven invalid form", profile.Kind(), raw)
			}
		}
	}
}
