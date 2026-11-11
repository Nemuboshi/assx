package lint

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"assx/internal/ass/renderer"
)

func htmlFixtureDoc(text string) renderer.Document {
	return defaultDocument("[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
		"Style: Default,Arial,40,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:00.00,0:00:02.00,Default,,0,0,0,," + text + "\n")
}

func htmlFixtureDocDialogues(texts ...string) renderer.Document {
	var b strings.Builder
	b.WriteString("[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n")
	b.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	b.WriteString("Style: Default,Arial,40,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n")
	b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for i, text := range texts {
		start := fmt.Sprintf("0:00:%02d.00", i)
		end := fmt.Sprintf("0:00:%02d.00", i+1)
		fmt.Fprintf(&b, "Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n", start, end, text)
	}
	return defaultDocument(b.String())
}

func TestHTMLTagSequence(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{`<b>A`, []string{"b"}},
		{`<{}b>A`, nil},
		{`<font face="{{Arial}">A`, []string{`font face="{{Arial}"`}},
		{`{<b>}`, nil},
		{`1 < 2`, nil},
		{`a < b > c`, []string{" b "}},
		{`<br>`, nil},
		{`</b>`, []string{"/b"}},
	}
	for _, tc := range cases {
		if got := htmlTagSequence(tc.text); !slices.Equal(got, tc.want) {
			t.Errorf("%q: got %#v, want %#v", tc.text, got, tc.want)
		}
	}
}

// TestSafeFixRejectsDialogueWithPossibleHTMLTag covers the four renderer
// reproductions from issue #1. The diagnostic must stay, but no SafeFix may
// survive and no edit may reach the file.
func TestSafeFixRejectsDialogueWithPossibleHTMLTag(t *testing.T) {
	cases := []struct {
		name      string
		text      string
		condition string
	}{
		{"empty block", `<{}b>A`, IssueEmptyOverrideBlock},
		{"no effect", `{\fs41}X<{\fs41}b>A`, IssueNoEffect},
		{"redundant style", `<{\fs40}b>A`, IssueRedundantStyleOverrides},
		{"repeated brace", `<font face="{{Arial}">A`, IssueRepeatedOpenBrace},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := htmlFixtureDoc(tc.text)
			findings := analyzeDocumentTest(doc)
			var found bool
			for _, diagnostic := range findings {
				if diagnostic.Condition != tc.condition {
					continue
				}
				found = true
				if diagnostic.FixSafety == SafeFix || len(diagnostic.Edits) != 0 || diagnostic.FixProof != nil {
					t.Errorf("HTML-sensitive edit kept a proof: %#v", diagnostic)
				}
			}
			if !found {
				t.Fatalf("condition %s is missing", tc.condition)
			}
			fixed, count, err := ApplyFixes(doc.Text, findings, false)
			if err != nil || count != 0 || fixed != doc.Text {
				t.Fatalf("HTML-sensitive edit was applied: count=%d err=%v", count, err)
			}
		})
	}
}

// TestHTMLBarrierKeepsUnrelatedDialoguesFixable checks that the barrier is
// scoped to the dialogue whose text changes its HTML interpretation.
func TestHTMLBarrierKeepsUnrelatedDialoguesFixable(t *testing.T) {
	doc := htmlFixtureDocDialogues(`{\b0\b0}<{}b>A`, `{\b0\b0}plain`)
	findings := analyzeDocumentTest(doc)
	fixed, count, err := ApplyFixes(doc.Text, findings, false)
	if err != nil || count != 2 {
		t.Fatalf("apply: count=%d err=%v", count, err)
	}
	if !strings.Contains(fixed, `<{}b>A`) {
		t.Errorf("HTML-sensitive empty-block edit was applied: %q", fixed)
	}
	if !strings.Contains(fixed, ",,plain") {
		t.Errorf("unrelated dialogue was not fixed: %q", fixed)
	}
}

// TestSafeFixKeepsAnUntouchedHTMLTag allows an edit that leaves the consumed
// tag sequence unchanged, including a line that already contains a real tag.
func TestSafeFixKeepsAnUntouchedHTMLTag(t *testing.T) {
	for _, text := range []string{`{\b0\b0}<b>A`, `{\b0\b0}a < b > c`} {
		doc := htmlFixtureDoc(text)
		findings := analyzeDocumentTest(doc)
		fixed, count, err := ApplyFixes(doc.Text, findings, false)
		if err != nil || count != 1 || fixed == doc.Text {
			t.Fatalf("%q: count=%d err=%v", text, count, err)
		}
		if !strings.Contains(fixed, text[len(`{\b0\b0}`):]) {
			t.Errorf("%q: tag text changed: %q", text, fixed)
		}
	}
}

func TestSafeFixKeepsOrdinaryLessThanText(t *testing.T) {
	for _, text := range []string{`{\b0\b0}1 < 2`, `{\b0\b0}no <br> here`, `{\b0\b0}<>`} {
		doc := htmlFixtureDoc(text)
		findings := analyzeDocumentTest(doc)
		fixed, count, err := ApplyFixes(doc.Text, findings, false)
		if err != nil || count == 0 || fixed == doc.Text {
			t.Fatalf("%q: count=%d err=%v", text, count, err)
		}
	}
}

func TestHTMLBarrierCoversEachVSFilterTarget(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	mod, _ := renderer.Standard(renderer.VSFilterMod)
	targets := []struct {
		name  string
		value AnalysisTargets
	}{
		{"xy-VSFilter primary", AnalysisTargets{Primary: xy}},
		{"VSFilterMod primary", AnalysisTargets{Primary: mod}},
		{"libass primary with xy-VSFilter", AnalysisTargets{Primary: libass, Compatibility: []renderer.Profile{xy}}},
	}
	for _, tc := range targets {
		t.Run(tc.name, func(t *testing.T) {
			doc := htmlFixtureDoc(`<{}b>A`)
			findings := analyzeTargetsTest(doc, tc.value)
			fixed, count, err := ApplyFixesForTargets(doc.Text, findings, false, tc.value)
			if err != nil || count != 0 || fixed != doc.Text {
				t.Fatalf("HTML-sensitive edit was applied: count=%d err=%v", count, err)
			}
		})
	}
}

func TestHTMLBarrierDoesNotApplyToLibassOnly(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	targets := AnalysisTargets{Primary: libass}
	doc := htmlFixtureDoc(`{\b0\b0}<b>A`)
	findings := analyzeTargetsTest(doc, targets)
	fixed, count, err := ApplyFixesForTargets(doc.Text, findings, false, targets)
	if err != nil || count != 1 || fixed == doc.Text {
		t.Fatalf("libass-only fix: count=%d err=%v", count, err)
	}
}
