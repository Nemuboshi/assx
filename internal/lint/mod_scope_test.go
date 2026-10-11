package lint

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestP09PreservesDefaultModValueValidation(t *testing.T) {
	for _, source := range []string{`{\z(bad)}A`, `{\frs(bad)}A`, `{\fsvp(bad)}A`, `{\fshp(bad)}A`, `{\ortho2}A`} {
		got := Analyze(ass.Dialogue{Text: source})
		if len(got) != 1 || got[0].ID != IssueVSFilterModTag {
			t.Errorf("default validation changed for %s: %#v", source, got)
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
