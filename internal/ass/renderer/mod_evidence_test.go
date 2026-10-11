package renderer

import "testing"

func TestModSimpleSignaturesAndBuildScope(t *testing.T) {
	for _, tag := range []string{"z", "frs", "fsvp", "fshp", "ortho"} {
		for _, raw := range []string{"\\" + tag, "\\" + tag + "1", "\\" + tag + "(1)"} {
			for _, feature := range []Feature{FeatureEnabled, FeatureDisabled, FeatureUnknown} {
				profile, err := New(VSFilterMod, Build{Mod: feature, Lua: FeatureDisabled})
				if err != nil {
					t.Fatal(err)
				}
				expr, source := first(t, raw)
				result := profile.Resolve(expr, source)
				if feature == FeatureEnabled {
					if result.Name != tag || result.Status != Matched || result.Signature != SignatureVerified || result.Citation == "" {
						t.Errorf("%s: %+v", raw, result)
					}
				} else if feature == FeatureUnknown {
					if result.Status != Conditional || result.Signature == SignatureVerified {
						t.Errorf("conditional %s: %+v", raw, result)
					}
				} else if result.Name == tag && result.Status == Matched {
					t.Errorf("disabled %s matched: %+v", raw, result)
				}
			}
		}
	}
}

func TestMovePinnedExhaustiveSignatures(t *testing.T) {
	for _, profile := range []Profile{profiles(t)[0], profiles(t)[1], profiles(t)[2]} {
		for _, tc := range []struct {
			raw  string
			want SignatureStatus
		}{
			{`\move(1,2,3,4)`, SignatureVerified},
			{`\move(1,2,3,4,5,6)`, SignatureVerified},
			{`\move(1,2,3,4,5,6,7,8)`, SignatureRejected},
			{`\move(1,2,3)`, SignatureRejected},
		} {
			expr, source := first(t, tc.raw)
			if got := profile.Resolve(expr, source); got.Signature != tc.want || got.Citation == "" {
				t.Errorf("%s %s: %+v", profile.Kind(), tc.raw, got)
			}
		}
	}
}
