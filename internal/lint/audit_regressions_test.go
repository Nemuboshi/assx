package lint

import (
	"reflect"
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestSafeFixRendererRegressions(t *testing.T) {
	cases := []struct {
		name, format, style, text, want string
	}{
		{"parenthesized color", "Name, PrimaryColour", "Default,&H00000000&", `{\1c&HFFFFFF&}A{\1c(&HFFFFFF&)}B`, `{\1c&HFFFFFF&}A{\1c(&HFFFFFF&)}B`},
		{"negative combined shadow", "Name, Shadow", "Default,1", `{\xshad-2\yshad-2\shad-2}A`, `{\shad-2}A`},
		{"unknown italic prefix", "Name, Italic", "Default,0", `{\i1}A{\iabc\i1}B`, `{\i1}A{\iabc\i1}B`},
		{"unknown before text", "Name, Italic", "Default,0", `{\i1\iabc\i1}B`, `{\i1\iabc\i1}B`},
		{"parenthesized style color", "Name, PrimaryColour", "Default,&H00FFFFFF&", `{\1c(&HFFFFFF&)}A`, `{\1c(&HFFFFFF&)}A`},
		{"lowercase override color prefix", "Name, PrimaryColour", "Default,&H00000000&", `{\1c&HFFFFFF&}A{\1c&hFFFFFF&}B`, `{\1c&HFFFFFF&}A{\1c&hFFFFFF&}B`},
		{"parenthesized alpha", "Name, PrimaryColour", "Default,&H00000000&", `{\1a&H80&}A{\1a(&H80&)}B`, `{\1a&H80&}A{\1a(&H80&)}B`},
		{"parenthesized transform color", "Name, PrimaryColour", "Default,&H00000000&", `{\pos(20,20)\1c&HFFFFFF&\t(\1c(&HFFFFFF&))}A`, `{\pos(20,20)\1c&HFFFFFF&\t(\1c(&HFFFFFF&))}A`},
		{"negative shadow transform", "Name, Shadow", "Default,1", `{\pos(20,20)\shad0\t(\shad-2)}A`, `{\pos(20,20)\shad0\t(\shad-2)}A`},
		{"font zero with leading space", "Name, Fontname", "Default,Go", `{\fn 0}A`, `{\fn 0}A`},
		{"font zero with trailing space", "Name, Fontname", "Default,Go", `{\fn0 }A`, "A"},
		{"parenthesized font zero reset", "Name, Fontname", "Default,Go", `{\fn( 0 )}A`, "A"},
		{"parenthesized empty bold reset", "Name, Bold", "Default,-1", `{\b( )}A`, `{\b( )}A`},
		{"parenthesized empty italic reset", "Name, Italic", "Default,-1", `{\i( )}A`, `{\i( )}A`},
		{"bold with trailing whitespace", "Name, Bold", "Default,-1", `{\b }A`, "A"},
		{"unicode whitespace before italic", "Name, Italic", "Default,0", "{\\i1}A{\\i\u30001}B", "{\\i1}A{\\i\u30001}B"},
		{"zero shadow after clamping", "Name, Shadow", "Default,1", `{\shad-2}A{\shad0}B`, `{\shad-2}AB`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := parseStyleOverrideText(test.format, test.style, test.text)
			fixed, _, err := ApplyFixes(doc.Text, AnalyzeDocument(doc), false)
			if err != nil {
				t.Fatal(err)
			}
			if got := ass.Parse(fixed).Dialogues[0].Text; got != test.want {
				t.Fatalf("fixed text = %q, want %q", got, test.want)
			}
			second, count, err := ApplyFixes(fixed, AnalyzeDocument(ass.Parse(fixed)), false)
			if err != nil || count != 0 || second != fixed {
				t.Fatalf("fix is not stable: (%q, %d, %v)", second, count, err)
			}
		})
	}
}

func TestLateVSFilterModTagsDisableAllDialogueFixes(t *testing.T) {
	for _, text := range []string{
		`{\fs20}A{\distort(1)}B`,
		`{\fs20}A{\t(\distort(1))}B`,
		`{\fs20}A{\unknown}B{\distort(1)}C`,
	} {
		doc := parseFontOverrideText(text)
		for _, diagnostics := range [][]Diagnostic{AnalyzeDocument(doc), AnalyzeRedundantStyleOverrides(doc)} {
			for _, diagnostic := range diagnostics {
				if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
					t.Fatalf("%s remains auto-fixable: %#v", text, diagnostic)
				}
			}
			fixed, count, err := ApplyFixes(doc.Text, diagnostics, true)
			if err != nil || count != 0 || fixed != doc.Text {
				t.Fatalf("extension dialogue changed: (%q, %d, %v)", fixed, count, err)
			}
		}
	}
}

func TestWhitespaceOnlyHeaderFixes(t *testing.T) {
	for _, whitespace := range []string{"", "   ", "\t \t"} {
		text := "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix:" + whitespace + "\nLayoutResX:" + whitespace + "\nLayoutResY:" + whitespace + "\n"
		doc := ass.Parse(text)
		fixed, count, err := ApplyFixes(text, AnalyzeHeaders(doc), true)
		if err != nil || count != 2 {
			t.Fatalf("whitespace %q: count=%d error=%v", whitespace, count, err)
		}
		for key, want := range map[string]string{"ycbcr matrix": "None", "layoutresx": "640", "layoutresy": "480"} {
			if got := ass.Parse(fixed).Headers[key].Value; got != want {
				t.Fatalf("fixed %s = %q, want %q", key, got, want)
			}
		}
	}
}

func TestEveryEventFormatHasItsOwnDiagnostic(t *testing.T) {
	standard := "Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:02.00,Default,,0,0,0,,first\n"
	custom := "  Format: Layer, Start, End, Style, Text\nDialogue: 0,0:00:00.00,0:00:02.00,Default,second\n"
	for _, test := range []struct {
		text  string
		lines []int
	}{
		{standard + custom, []int{4}},
		{custom + standard, []int{2}},
		{custom + standard + custom, []int{2, 6}},
	} {
		diagnostics := diagnosticsForIDs(AnalyzeDocument(ass.Parse("[Events]\n"+test.text)), IssueEventFormat)
		var lines []int
		for _, diagnostic := range diagnostics {
			lines = append(lines, diagnostic.Line)
			if diagnostic.Column != 3 {
				t.Fatalf("wrong format column: %#v", diagnostic)
			}
		}
		if !reflect.DeepEqual(lines, test.lines) {
			t.Fatalf("format lines = %v, want %v", lines, test.lines)
		}
	}
}

func TestStyleFallbackAfterNamedResetRequiresRendererAgreement(t *testing.T) {
	definitions := "Style: Default,Arial,20,0,0\nStyle: Other,Courier New,24,-1,-1\n"
	for _, text := range []string{`{\rOther\b}A`, `{\rOther\b2}A`, `{\rOther\i}A`, `{\rOther\i2}A`, `{\rOther\fn0}A`, `{\rOther\fn(0)}A`, `{\rOther\fs0}A`, `{\rOther\fs}A`} {
		doc := parseStyleDefinitionsText("Name, Fontname, Fontsize, Bold, Italic", definitions, "Default", text)
		fixed, count, err := ApplyFixes(doc.Text, AnalyzeDocument(doc), false)
		if err != nil || count != 0 || fixed != doc.Text {
			t.Fatalf("renderer-dependent fallback %s changed: (%q, %d, %v)", text, fixed, count, err)
		}
	}
}

func TestRedundantStyleMetadataAndStateIsolation(t *testing.T) {
	doc := parseStyleDefinitionsText("Name, Fontsize", "Style: Default,20\nStyle: Other,24\n", "Default", `{\rOther\fs24}A`)
	doc = ass.Parse(doc.Text + "Dialogue: 0,0:00:00.00,0:00:02.00,Default,,0,0,0,,{\\fs20}B\n" +
		"Dialogue: 0,0:00:00.00,0:00:02.00,Default,,0,0,0,,{\\fs24}C\n")
	diagnostics := AnalyzeRedundantStyleOverrides(doc)
	if len(diagnostics) != 2 {
		t.Fatalf("shared style state leaked between dialogues: %#v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		rule := Rules[IssueRedundantStyleOverrides]
		if diagnostic.Title != rule.Title || diagnostic.Description != rule.Description || diagnostic.Fix != rule.Fix {
			t.Fatalf("missing rule metadata: %#v", diagnostic)
		}
	}
	fixed, _, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil || !strings.Contains(fixed, `,,{\fs24}C`) {
		t.Fatalf("nonredundant third dialogue changed: %q / %v", fixed, err)
	}
}
