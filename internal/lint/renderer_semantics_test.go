package lint

import (
	"os"
	"path/filepath"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestRendererScopedASS006DoesNotInventSafeFix(t *testing.T) {
	dialogue := ass.Dialogue{
		Text: `{\pos(1,2,3)\pos(4,5)}A`,
		Line: 1,
	}
	for _, tc := range []struct {
		kind renderer.Kind
		want int
	}{
		{renderer.Libass, 0},
		{renderer.XYVSFilter, 0},
		{renderer.VSFilterMod, 0},
	} {
		var profile renderer.Profile
		var err error
		if tc.kind == renderer.VSFilterMod {
			profile, err = renderer.New(tc.kind, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
		} else {
			profile, err = renderer.Standard(tc.kind)
		}
		if err != nil {
			t.Fatal(err)
		}
		findings := AnalyzeNoEffectsForRenderer(dialogue, profile)
		if len(findings) != tc.want {
			t.Errorf("%s ASS006=%#v, want %d", tc.kind, findings, tc.want)
		}
		for _, d := range findings {
			if d.ID != IssueNoEffect || d.FixSafety != "" || len(d.Edits) != 0 {
				t.Errorf("%s local observation promoted to a fix: %#v", tc.kind, d)
			}
		}
	}
}

func TestRendererScopedASS013DoesNotInventSafeFix(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "testdata", "redundant-font.ass"))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	findings := AnalyzeRedundantStyleOverridesForRenderer(ass.Parse(string(content)), profile)
	if len(findings) != 1 {
		t.Fatalf("scoped style findings = %#v", findings)
	}
	if findings[0].ID != IssueRedundantStyleOverrides || findings[0].FixSafety != "" || len(findings[0].Edits) != 0 {
		t.Fatalf("ASS013 must not promote single-renderer equivalence to a SafeFix: %#v", findings[0])
	}
}
