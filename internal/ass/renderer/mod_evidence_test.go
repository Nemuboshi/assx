package renderer

import "testing"

func TestModSimpleSignatures(t *testing.T) {
	profile, err := New(VSFilterMod)
	if err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{"z", "frs", "fsvp", "fshp", "ortho"} {
		for _, raw := range []string{"\\" + tag, "\\" + tag + "1", "\\" + tag + "(1)"} {
			expr, source := first(t, raw)
			result := profile.Resolve(expr, source)
			if result.Name != tag || result.Status != Matched || result.Signature != SignatureVerified || result.Citation == "" {
				t.Errorf("%s: %+v", raw, result)
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
