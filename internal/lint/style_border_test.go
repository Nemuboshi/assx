package lint

import (
	"fmt"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

const borderStyleFormat = "Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding"

func borderStyleDocument(border, scaleX, scaleY, outline string) string {
	return "[Script Info]\nScriptType: v4.00+\n[V4+ Styles]\nFormat: " + borderStyleFormat + "\n" +
		fmt.Sprintf("Style: Default,Arial,32,&H00FFFFFF,&H00FFFFFF,&H00000000,&H80000000,0,0,0,0,%s,%s,0,0,%s,%s,0,2,20,20,20,1\n", scaleX, scaleY, border, outline)
}

func TestStyleBorderCompatibilityRules(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	mod, _ := renderer.Standard(renderer.VSFilterMod)
	disabledMod, _ := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureDisabled, Lua: renderer.FeatureDisabled})
	cases := []struct {
		name, border, x, y, outline string
		profiles                    []renderer.Profile
		id                          string
		count                       int
	}{
		{name: "default mode difference", border: "4", id: IssueStyleBorderMode, count: 1},
		{name: "mode with Mod", border: "4", profiles: []renderer.Profile{libass, mod}, id: IssueStyleBorderMode, count: 1},
		{name: "reversed mode pair", border: "4", profiles: []renderer.Profile{xy, libass}, id: IssueStyleBorderMode, count: 1},
		{name: "two VSFilters share outline mode", border: "4", profiles: []renderer.Profile{xy, mod}},
		{name: "single renderer", border: "4", profiles: []renderer.Profile{libass}},
		{name: "duplicate profiles", border: "4", profiles: []renderer.Profile{libass, xy, libass}, id: IssueStyleBorderMode, count: 1},
		{name: "all mode pairs", border: "4", profiles: []renderer.Profile{libass, xy, mod}, id: IssueStyleBorderMode, count: 2},
		{name: "signed and padded integer", border: " +004 ", id: IssueStyleBorderMode, count: 1},
		{name: "fractional integer is separate", border: "4.5"},
		{name: "malformed integer", border: "4junk"},
		{name: "overflow integer", border: "4294967300"},
		{name: "unmodeled hex form", border: "&H4"},
		{name: "unicode numeric space", border: "\u00a04"},
		{name: "normal outline", border: "1"},
		{name: "outline fallback zero", border: "0"},
		{name: "outline fallback two", border: "2"},
		{name: "ordinary box", border: "3", profiles: []renderer.Profile{libass, xy, mod}},
		{name: "traditional box scaling", border: "3", x: "200"},
		{name: "horizontal box scaling", border: "3", x: "200", profiles: []renderer.Profile{libass, mod}, id: IssueStyleBoxGeometry, count: 1},
		{name: "vertical box scaling", border: "3", y: "50", profiles: []renderer.Profile{xy, mod}, id: IssueStyleBoxGeometry, count: 1},
		{name: "reversed box pair", border: "3", x: "200", profiles: []renderer.Profile{mod, libass}, id: IssueStyleBoxGeometry, count: 1},
		{name: "scale exponent", border: "3", x: "2e2", profiles: []renderer.Profile{xy, mod}, id: IssueStyleBoxGeometry, count: 1},
		{name: "all box pairs", border: "3", x: "200", profiles: []renderer.Profile{libass, xy, mod}, id: IssueStyleBoxGeometry, count: 2},
		{name: "scale exception has no build guard", border: "3", x: "200", profiles: []renderer.Profile{xy, disabledMod}, id: IssueStyleBoxGeometry, count: 1},
		{name: "Mod builds share box scaling", border: "3", x: "200", profiles: []renderer.Profile{mod, disabledMod}},
		{name: "no unshifted box", border: "3", x: "200", outline: "0", profiles: []renderer.Profile{libass, mod}},
		{name: "negative scale", border: "3", x: "-200", profiles: []renderer.Profile{libass, mod}},
		{name: "zero scale", border: "3", y: "0", profiles: []renderer.Profile{libass, mod}},
		{name: "unknown scale", border: "3", x: "200junk", profiles: []renderer.Profile{libass, mod}},
		{name: "scale precision is separate", border: "3", x: "100.0000001", profiles: []renderer.Profile{libass, mod}},
		{name: "nonfinite scale", border: "3", x: "1e999", profiles: []renderer.Profile{libass, mod}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x, y, outline := tc.x, tc.y, tc.outline
			if x == "" {
				x = "100"
			}
			if y == "" {
				y = "100"
			}
			if outline == "" {
				outline = "2"
			}
			source := borderStyleDocument(tc.border, x, y, outline)
			findings := diagnosticsForIDs(AnalyzeDocumentCompatibility(ass.Parse(source), tc.profiles), IssueStyleBoxGeometry, IssueStyleBorderMode)
			if len(findings) != tc.count {
				t.Fatalf("got %d findings, want %d: %#v", len(findings), tc.count, findings)
			}
			for _, finding := range findings {
				if finding.ID != tc.id || finding.Severity != Warning || finding.Description != Rules[tc.id].Description || finding.Fix != Rules[tc.id].Fix {
					t.Fatalf("wrong rule metadata: %#v", finding)
				}
				if finding.FixSafety != "" || len(finding.Edits) != 0 || finding.FixProof != nil {
					t.Fatalf("compatibility warning offers a fix: %#v", finding)
				}
				if finding.Renderer == "" || len(finding.Sources) != 2 || finding.SourceSpan == nil || source[finding.SourceSpan.Start:finding.SourceSpan.End] != strings.TrimSpace(tc.border) {
					t.Fatalf("missing scope, evidence, or field span: %#v", finding)
				}
			}
		})
	}
}

func TestStyleBorderWarningsUsePhysicalFieldsAndNeverEdit(t *testing.T) {
	source := strings.ReplaceAll(borderStyleDocument(" 4 ", "100", "100", "2"), "\n", "\r\n")
	doc := ass.Parse(source)
	warnings := diagnosticsForIDs(AnalyzeDocument(doc), IssueStyleBorderMode)
	if len(warnings) != 1 {
		t.Fatalf("default analysis lost mode warning: %#v", warnings)
	}
	d := warnings[0]
	line := strings.Split(source, "\r\n")[4]
	if d.Line != 5 || d.Column != strings.Index(line, ", 4 ,")+3 || d.Tag != "" || d.Field != "document/style-border-mode" {
		t.Fatalf("wrong physical field: %#v", d)
	}
	for _, unsafe := range []bool{false, true} {
		fixed, count, err := ApplyFixes(source, warnings, unsafe)
		if err != nil || count != 0 || fixed != source {
			t.Fatalf("warning changed source: count=%d err=%v", count, err)
		}
	}
	if findings := diagnosticsForIDs(AnalyzeDocumentForRenderer(doc, DefaultAnalysisTargets().Primary), IssueStyleBorderMode, IssueStyleBoxGeometry); len(findings) != 0 {
		t.Fatalf("single-profile validation reported a comparison: %#v", findings)
	}
}

func TestStyleBorderComparisonKeepsIndependentLayoutFindings(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	mod, _ := renderer.Standard(renderer.VSFilterMod)
	for _, tc := range []struct{ border, scale, id string }{
		{"3", "200", IssueStyleBoxGeometry},
		{"4", "100", IssueStyleBorderMode},
	} {
		source := borderStyleDocument(tc.border, tc.scale, "100", "2")
		source = strings.Replace(source, borderStyleFormat, borderStyleFormat+", Justify", 1)
		source = strings.TrimSuffix(source, "\n") + ",0\n"
		findings := AnalyzeDocumentCompatibility(ass.Parse(source), []renderer.Profile{libass, mod})
		if len(diagnosticsForIDs(findings, tc.id)) != 1 || len(diagnosticsForIDs(findings, IssueRendererDiff)) != 1 {
			t.Fatalf("layout and BorderStyle differences must remain separate: %#v", findings)
		}
	}
}

func TestStyleBorderCompatibilitySkipsUnresolvedRows(t *testing.T) {
	base := borderStyleDocument("4", "100", "100", "2")
	cases := []string{
		strings.Replace(base, "[V4+ Styles]", "[V4 Styles]", 1),
		strings.Replace(base, "[V4+ Styles]", "[V4++ Styles]", 1),
		strings.Replace(base, "Style:", "style:", 1),
		strings.Replace(base, borderStyleFormat, "BorderStyle, "+borderStyleFormat, 1),
		strings.Replace(base, "BorderStyle, Outline", "Outline, BorderStyle", 1),
		strings.Replace(base, ",20,20,20,1\n", "\n", 1),
	}
	for _, source := range cases {
		findings := diagnosticsForIDs(AnalyzeDocumentCompatibility(ass.Parse(source), nil), IssueStyleBorderMode, IssueStyleBoxGeometry)
		if len(findings) != 0 {
			t.Fatalf("unresolved or different layouts produced a field comparison: %#v", findings)
		}
	}
}
