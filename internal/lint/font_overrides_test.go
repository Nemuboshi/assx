package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestRedundantFontOverridesSafeFix(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "redundant-font.ass")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := ass.Parse(string(data))
	diagnostics := AnalyzeDocument(doc)
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantFontOverrides {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].FixSafety != SafeFix || len(diagnostics[0].Edits) != 3 {
		t.Fatalf("redundant font finding = %#v", diagnostics[0])
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.Contains(fixed, "Default,alpha beta\n") {
		t.Fatalf("fixed document (%d edits):\n%s", count, fixed)
	}
	if remaining := AnalyzeRedundantFontOverrides(ass.Parse(fixed)); len(remaining) != 0 {
		t.Fatalf("fixed document still has redundant font overrides: %#v", remaining)
	}
}

func TestRedundantFontOverrideStateTracking(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "repeated leading blocks and same value later", text: `{\fnCourier New}{\fnArial}A{\fnArial}B`, want: true},
		{name: "leading assignments settle on style", text: `{\fs24\fs20}A{\fs20}B`, want: true},
		{name: "midline same as current style", text: `A{\fs20}B`, want: true},
		{name: "empty font name resets to style", text: `{\fn}A`, want: true},
		{name: "zero font name resets to style", text: `{\fn0}A`, want: true},
		{name: "empty font tags reset to style", text: `{\fs}{\fscx}{\fscy}{\fsp}{\b}{\i}A`, want: true},
		{name: "zero font size resets to style", text: `{\fs0}A`, want: true},
		{name: "relative font size is skipped", text: `{\fs+10}A`},
		{name: "bold assignments settle on style", text: `{\b1\b0}A`, want: true},
		{name: "scale assignments settle on style", text: `{\fscx110\fscx100}A`, want: true},
		{name: "italic y scale and spacing settle on style", text: `{\i1\i0\fscy120\fscy100\fsp2\fsp0}A`, want: true},
		{name: "font state differs at first text", text: `{\fnCourier New}A{\fnArial}B`},
		{name: "font changes in the middle", text: `{\fnArial}A{\fnCourier New}B{\fnArial}C`},
		{name: "style reset is unsupported", text: `{\fnArial}A{\rOther}B`},
		{name: "font transform is unsupported", text: `{\fnArial}A{\t(0,500,\fs24)}B`},
		{name: "rotation is not modeled", text: `{\frz0}A`},
		{name: "encoding is not modeled", text: `{\fe1}A`},
		{name: "font family comparison is exact", text: `{\fnarial}A`},
		{name: "weight values are restricted", text: `{\b100}A`},
		{name: "invalid numeric values are skipped", text: `{\fs20px}A`},
		{name: "no visible text", text: `{\fnArial}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := parseFontOverrideText(test.text)
			got := len(AnalyzeRedundantFontOverrides(doc)) != 0
			if got != test.want {
				t.Fatalf("found redundant font override = %t, want %t", got, test.want)
			}
		})
	}
}

func TestRedundantFontFixPreservesOtherTags(t *testing.T) {
	doc := parseFontOverrideText(`{\c&H00FF00&\fnCourier New\fnArial}A{\fnArial}B`)
	diagnostics := AnalyzeDocument(doc)
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantFontOverrides {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.Contains(fixed, `Dialogue: 0, 0:00:00.00, 0:00:02.00, Default,{\c&H00FF00&}AB`) {
		t.Fatalf("other formatting tag was not preserved (%d fixes):\n%s", count, fixed)
	}
}

func TestRedundantFontRuleRejectsAmbiguousStyles(t *testing.T) {
	text := fontOverrideDocument(`{\fnArial}A`, "")
	text = strings.Replace(text, "Style: Default, Arial, 20, 0, 0, 100, 100, 0\n", "Style: Default, Arial, 20, 0, 0, 100, 100, 0\nStyle: Default, Arial, 20, 0, 0, 100, 100, 0\n", 1)
	if diagnostics := AnalyzeRedundantFontOverrides(ass.Parse(text)); len(diagnostics) != 0 {
		t.Fatalf("ambiguous style produced findings: %#v", diagnostics)
	}
}

func TestRedundantFontRuleRequiresExactStyleReference(t *testing.T) {
	text := strings.Replace(fontOverrideDocument(`{\fnArial}A`, ""), "Default,{\\fnArial}", "default,{\\fnArial}", 1)
	if diagnostics := AnalyzeRedundantFontOverrides(ass.Parse(text)); len(diagnostics) != 0 {
		t.Fatalf("case-mismatched style reference produced findings: %#v", diagnostics)
	}
}

func parseFontOverrideText(text string) ass.Document {
	return ass.Parse(fontOverrideDocument(text, ""))
}

func fontOverrideDocument(text, extraStyle string) string {
	return "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, Bold, Italic, ScaleX, ScaleY, Spacing\n" +
		"Style: Default, Arial, 20, 0, 0, 100, 100, 0\n" + extraStyle +
		"[Events]\nFormat: Layer, Start, End, Style, Text\n" +
		"Dialogue: 0, 0:00:00.00, 0:00:02.00, Default," + text + "\n"
}
