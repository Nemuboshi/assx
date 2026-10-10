package lint

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func scopedProfiles(t *testing.T) (renderer.Profile, renderer.Profile, renderer.Profile, renderer.Profile, renderer.Profile) {
	t.Helper()
	lib, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	xy, err := renderer.Standard(renderer.XYVSFilter)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := renderer.Standard(renderer.VSFilterMod)
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureDisabled})
	if err != nil {
		t.Fatal(err)
	}
	return lib, xy, enabled, unknown, disabled
}

func hasIssue(ds []Diagnostic, id string) bool {
	for _, d := range ds {
		if d.ID == id {
			return true
		}
	}
	return false
}

func TestScopedValidityPreservesDefaultStrictness(t *testing.T) {
	lib, xy, enabled, unknown, disabled := scopedProfiles(t)
	for _, tt := range []struct {
		name                string
		profile             renderer.Profile
		invalid, unresolved bool
	}{
		{"libass", lib, true, false},
		{"xy", xy, true, false},
		{"mod enabled", enabled, false, false},
		{"mod unknown", unknown, false, true},
		{"mod disabled", disabled, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dialogue := ass.Dialogue{Line: 1, Text: `{\pos(1,2,3)}A`}
			ds := AnalyzeForRenderer(dialogue, tt.profile)
			if hasIssue(ds, IssueArgumentCount) != tt.invalid ||
				hasIssue(ds, IssueRendererUnresolved) != tt.unresolved {
				t.Fatalf("scoped diagnostics: %#v", ds)
			}
			for _, d := range ds {
				if d.Renderer != tt.profile.Kind().String() || d.Edits != nil || d.FixSafety != "" {
					t.Fatalf("unexpected scope or automatic edit: %#v", d)
				}
			}
		})
	}
	if !hasIssue(Analyze(ass.Dialogue{Line: 1, Text: `{\pos(1,2,3)}A`}), IssueArgumentCount) {
		t.Fatal("default parser silently accepted VSFilterMod-only arity")
	}
}

func TestScopedPrefixCollisionsAndJunk(t *testing.T) {
	lib, xy, mod, unknown, _ := scopedProfiles(t)
	for _, raw := range []string{`\fsvp6`, `\frs10`, `\blend(add)`} {
		for _, traditional := range []renderer.Profile{lib, xy} {
			got := AnalyzeForRenderer(ass.Dialogue{Line: 3, Text: "{" + raw + "}X"}, traditional)
			if !hasIssue(got, IssueInvalidValue) {
				t.Errorf("%s %s lost invalid consumed-prefix value: %#v", traditional.Kind(), raw, got)
			}
			if hasIssue(got, IssueUnknownTag) || hasIssue(got, IssueVSFilterModTag) {
				t.Errorf("%s %s confused shadowing with global name lookup: %#v", traditional.Kind(), raw, got)
			}
		}
		got := AnalyzeForRenderer(ass.Dialogue{Line: 3, Text: "{" + raw + "}X"}, mod)
		if hasIssue(got, IssueInvalidValue) || hasIssue(got, IssueUnknownTag) || hasIssue(got, IssueVSFilterModTag) {
			t.Errorf("mod %s uses the wrong command: %#v", raw, got)
		}
		if got := AnalyzeForRenderer(ass.Dialogue{Line: 3, Text: "{" + raw + "}X"}, unknown); !hasIssue(got, IssueRendererUnresolved) {
			t.Errorf("unknown mod capabilities treated %s as confirmed: %#v", raw, got)
		}
	}
	for _, profile := range []renderer.Profile{lib, xy, mod} {
		ds := AnalyzeForRenderer(ass.Dialogue{Line: 1, Text: `{\fs20junk}A`}, profile)
		if !hasIssue(ds, IssueOverrideJunk) {
			t.Errorf("%s missing consumed suffix diagnostic: %#v", profile.Kind(), ds)
		}
	}
}

func TestScopedMalformedFormsNestedAndState(t *testing.T) {
	lib, xy, mod, _, _ := scopedProfiles(t)
	for _, profile := range []renderer.Profile{lib, xy, mod} {
		for _, raw := range []string{`{\clip(1,2,3)}A`, `{\iclip(1,2,3)}A`} {
			if ds := AnalyzeForRenderer(ass.Dialogue{Line: 1, Text: raw}, profile); !hasIssue(ds, IssueArgumentCount) {
				t.Errorf("%s malformed clip %q: %#v", profile.Kind(), raw, ds)
			}
		}
	}
	for _, profile := range []renderer.Profile{lib, xy} {
		got := AnalyzeForRenderer(ass.Dialogue{Line: 1, Text: `{\t(0,100,\pos(1,2,3))}A`}, profile)
		if !hasIssue(got, IssueArgumentCount) {
			t.Errorf("%s nested invalid position escaped validation: %#v", profile.Kind(), got)
		}
	}
	for _, tt := range []struct {
		profile      renderer.Profile
		wantNoEffect bool
	}{
		{lib, false}, {xy, false}, {mod, true},
	} {
		got := AnalyzeForRenderer(ass.Dialogue{Line: 1, Text: `{\pos(1,2,3)\pos(4,5)}A`}, tt.profile)
		if hasIssue(got, IssueNoEffect) != tt.wantNoEffect {
			t.Errorf("%s first-wins semantics: %#v", tt.profile.Kind(), got)
		}
	}
}

func TestScopedDocumentConsumersAndJSON(t *testing.T) {
	lib, _, mod, _, _ := scopedProfiles(t)
	source := "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nLayoutResX: 1920\nLayoutResY: 1080\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		`Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\pos(1,2,3)\rMissing\k200}Text` + "\n"
	doc := ass.Parse(source)
	traditional := AnalyzeDocumentForRenderer(doc, lib)
	modern := AnalyzeDocumentForRenderer(doc, mod)
	for _, tt := range []struct {
		name     string
		ds       []Diagnostic
		hasArity bool
	}{
		{"libass", traditional, true}, {"VSFilterMod", modern, false},
	} {
		if hasIssue(tt.ds, IssueArgumentCount) != tt.hasArity ||
			!hasIssue(tt.ds, IssueUndefinedStyle) || !hasIssue(tt.ds, IssueKaraoke) {
			t.Fatalf("%s document checks missing: %#v", tt.name, tt.ds)
		}
		for _, d := range tt.ds {
			if d.FixSafety != "" || len(d.Edits) != 0 {
				t.Fatalf("%s single-renderer finding invented fix: %#v", tt.name, d)
			}
		}
		b, err := json.Marshal(tt.ds)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"renderer":"`+tt.name+`"`) {
			t.Errorf("%s renderer context missing from JSON: %s", tt.name, b)
		}
	}
	before, err := json.Marshal(AnalyzeDocument(doc))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(before), `"renderer"`) {
		t.Fatalf("default JSON schema changed: %s", before)
	}
	if !reflect.DeepEqual(modern, AnalyzeDocumentForRenderer(doc, mod)) {
		t.Fatal("renderer scoped diagnostic ordering is nondeterministic")
	}
}

func TestScopedDrawingAndSyntaxSurviveMigration(t *testing.T) {
	lib, _, _, _, _ := scopedProfiles(t)
	ds := AnalyzeForRenderer(ass.Dialogue{Line: 1, Text: `{\p1}m 0 {\fs20}l 3{\p0}X{garbage}{\fnArial`}, lib)
	if !hasIssue(ds, IssueMalformedDrawing) || !hasIssue(ds, IssueOverrideJunk) ||
		!hasIssue(ds, IssueUnterminatedBlock) {
		t.Fatalf("lost drawing/override syntax checks: %#v", ds)
	}
}

func BenchmarkAnalyzeDocumentRenderer(b *testing.B) {
	const source = "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		`Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\fs30\pos(1,2)\bord2}Hello{\fs32}world` + "\n"
	doc := ass.Parse(source)
	profile, err := renderer.Standard(renderer.Libass)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("default", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = AnalyzeDocument(doc)
		}
	})
	b.Run("scoped", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = AnalyzeDocumentForRenderer(doc, profile)
		}
	})
}

func TestScopedDiagnosticLocationsAndCertainty(t *testing.T) {
	lib, _, mod, unknown, _ := scopedProfiles(t)
	for _, tc := range []struct {
		profile  renderer.Profile
		source   string
		id, tag  string
		severity Severity
	}{
		{lib, `{\pos(1,2,3)}A`, IssueArgumentCount, "pos", Error},
		{mod, `{\pos(1,2,3,4)}A`, IssueArgumentCount, "pos", Error},
		{unknown, `{\pos(1,2,3)}A`, IssueRendererUnresolved, "pos", Warning},
		{lib, `{\fsvp6}A`, IssueInvalidValue, "fs", Error},
		{mod, `{\posBogus}A`, IssueRendererUnresolved, "pos", Warning},
		{lib, `{\pos(1,2}A`, IssueRendererUnresolved, "pos", Warning},
	} {
		ds := AnalyzeForRenderer(ass.Dialogue{Line: 7, Text: tc.source}, tc.profile)
		found := false
		for _, d := range ds {
			if d.ID != tc.id {
				continue
			}
			found = true
			if d.Line != 7 || d.Column != 3 || d.Tag != tc.tag || d.Severity != tc.severity {
				t.Errorf("%s %q mislocated diagnostic: %#v", tc.profile.Kind(), tc.source, d)
			}
		}
		if !found {
			t.Errorf("%s %q missing %s: %#v", tc.profile.Kind(), tc.source, tc.id, ds)
		}
	}
}

func TestScopedExistingConsumerJSONContext(t *testing.T) {
	_, xy, _, _, _ := scopedProfiles(t)
	got := AnalyzeNoEffectsForRenderer(ass.Dialogue{
		Line: 1, Text: `{\pos(1,2)\pos(3,4)}A`,
	}, xy)
	if len(got) != 1 || got[0].Renderer != xy.Kind().String() {
		t.Fatalf("renderer-local ASS006 omitted scope: %#v", got)
	}
	for _, d := range got {
		if d.Edits != nil || d.FixSafety != "" {
			t.Fatalf("renderer-local observation claimed an edit: %#v", d)
		}
	}
}
