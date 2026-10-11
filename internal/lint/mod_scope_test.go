package lint

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestP09PreservesDefaultModValueValidation(t *testing.T) {
	for _, tc := range []struct {
		source        string
		invalidPrefix bool
	}{
		{`{\z(bad)}A`, false},
		{`{\frs(bad)}A`, true},
		{`{\fsvp(bad)}A`, true},
		{`{\fshp(bad)}A`, true},
		{`{\ortho2}A`, false},
	} {
		got := Analyze(ass.Dialogue{Text: tc.source})
		if !hasIssue(got, IssueVSFilterModTag) || hasIssue(got, IssueInvalidValue) != tc.invalidPrefix {
			t.Errorf("default portability/prefix validation changed for %s: %#v", tc.source, got)
		}
		for _, diagnostic := range got {
			if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 || diagnostic.FixProof != nil {
				t.Errorf("Mod-only source supplied a default fix: %#v", diagnostic)
			}
		}
	}
}

func TestP09ModOrthoFallbackIsNotOutOfRange(t *testing.T) {
	profile, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	if err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range AnalyzeForRenderer(ass.Dialogue{Text: `{\ortho2}A`}, profile) {
		if diagnostic.ID == IssueInvalidValue {
			t.Fatalf("original-Style fallback rejected: %#v", diagnostic)
		}
		if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
			t.Fatalf("single profile supplied a fix: %#v", diagnostic)
		}
	}
}
