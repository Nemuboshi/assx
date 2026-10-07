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

func TestRedundantStyleOverrideProperties(t *testing.T) {
	cases := []struct {
		name   string
		format string
		style  string
		text   string
	}{
		{name: "border", format: "Name, Outline", style: "Default,2", text: `{\bord2}A`},
		{name: "x border alias", format: "Name, Outline", style: "Default,2", text: `{\xbord2}A`},
		{name: "y border alias", format: "Name, Outline", style: "Default,2", text: `{\ybord2}A`},
		{name: "shadow", format: "Name, Shadow", style: "Default,1", text: `{\shad1}A`},
		{name: "x shadow alias", format: "Name, Shadow", style: "Default,1", text: `{\xshad1}A`},
		{name: "y shadow alias", format: "Name, Shadow", style: "Default,1", text: `{\yshad1}A`},
		{name: "primary color split from alpha", format: "Name, PrimaryColour", style: "Default,&H80FFFFFF&", text: `{\1c&HFFFFFF&}A`},
		{name: "secondary color", format: "Name, SecondaryColour", style: "Default,&H00112233&", text: `{\2c&H112233&}A`},
		{name: "outline color", format: "Name, OutlineColour", style: "Default,&H00010203&", text: `{\3c&H010203&}A`},
		{name: "back color", format: "Name, BackColour", style: "Default,&H00040506&", text: `{\4c&H040506&}A`},
		{name: "alpha", format: "Name, PrimaryColour, SecondaryColour, OutlineColour, BackColour", style: "Default,&H00FFFFFF&,&H00000000&,&H00112233&,&H00445566&", text: `{\alpha&H00&}A`},
		{name: "rotation alias", format: "Name, Angle", style: "Default,0", text: `{\fr0}A`},
		{name: "rotation z", format: "Name, Angle", style: "Default,0", text: `{\frz0}A`},
		{name: "underline", format: "Name, Underline", style: "Default,-1", text: `{\u1}A`},
		{name: "strikeout", format: "Name, StrikeOut", style: "Default,0", text: `{\s0}A`},
		{name: "encoding", format: "Name, Encoding", style: "Default,1", text: `{\fe1}A`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := parseStyleOverrideText(test.format, test.style, test.text)
			diagnostics := AnalyzeRedundantFontOverrides(doc)
			if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantFontOverrides {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
			if diagnostics[0].Severity != Suggestion || diagnostics[0].FixSafety != SafeFix {
				t.Fatalf("finding metadata = %#v", diagnostics[0])
			}
			if rule := Rules[IssueRedundantFontOverrides]; rule.Title != "Overrides match style" || rule.Severity != Suggestion || rule.FixSafety != SafeFix {
				t.Fatalf("ASS013 rule metadata = %#v", rule)
			}
		})
	}
}

func TestRedundantStyleOverrideRestorationAndOverlap(t *testing.T) {
	format, style := "Name, Outline", "Default,2"
	if diagnostics := AnalyzeDocument(parseStyleOverrideText(format, style, `{\bord4}A{\bord2}B`)); len(diagnostics) != 0 {
		t.Fatalf("state restoration was reported: %#v", diagnostics)
	}
	mixedAlpha := parseStyleOverrideText(
		"Name, PrimaryColour, SecondaryColour, OutlineColour, BackColour",
		"Default,&H00FFFFFF&,&H80000000&,&H00112233&,&H00445566&",
		`{\alpha&H00&}A`,
	)
	if diagnostics := AnalyzeRedundantFontOverrides(mixedAlpha); len(diagnostics) != 0 {
		t.Fatalf("partially matching alpha tag was reported: %#v", diagnostics)
	}

	doc := parseStyleOverrideText(format, style, `{\bord4\bord2}A`)
	diagnostics := AnalyzeDocument(doc)
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantFontOverrides {
		t.Fatalf("redundant sequence diagnostics = %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil || count != 1 || !strings.Contains(fixed, "Default,A\n") {
		t.Fatalf("sequence fix = (%q, %d, %v)", fixed, count, err)
	}

	overlap := AnalyzeDocument(parseStyleOverrideText(format, style, `{\bord2\bord2}A`))
	if len(overlap) != 1 || overlap[0].ID != IssueRedundantFontOverrides {
		t.Fatalf("ASS006/ASS013 overlap was not assigned to ASS013: %#v", overlap)
	}
}

func TestRedundantStyleOverridesDoNotModelUnsupportedProperties(t *testing.T) {
	tags := []string{`{\blur1}A`, `{\be1}A`, `{\frx0}A`, `{\fry0}A`, `{\fax0}A`, `{\fay0}A`, `{\clip(0,0,1,1)}A`, `{\p1}A`, `{\pbo0}A`}
	for _, text := range tags {
		t.Run(text, func(t *testing.T) {
			doc := parseStyleOverrideText("Name, Outline", "Default,2", text)
			if diagnostics := AnalyzeRedundantFontOverrides(doc); len(diagnostics) != 0 {
				t.Fatalf("unsupported property produced ASS013: %#v", diagnostics)
			}
		})
	}
}

func TestSameValueAssignmentsKeepPreviousEffectiveOwner(t *testing.T) {
	cases := []struct {
		name   string
		format string
		style  string
		text   string
		want   string
	}{
		{
			name: "font size", format: "Name, Fontsize", style: "Default,30",
			text: `{\fs20\fs20}A`, want: `Default,{\fs20}A`,
		},
		{
			name: "multi-slot border", format: "Name, Outline", style: "Default,3",
			text: `{\xbord2\ybord2\bord2}A`, want: `Default,{\xbord2\ybord2}A`,
		},
		{
			name: "overwritten before use", format: "Name, Fontsize", style: "Default,30",
			text: `{\fs10\fs20}A`, want: `Default,{\fs20}A`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := parseStyleOverrideText(test.format, test.style, test.text)
			diagnostics := AnalyzeDocument(doc)
			if len(diagnostics) != 1 || diagnostics[0].ID != IssueNoEffect || diagnostics[0].FixSafety != SafeFix {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
			fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
			if err != nil || count != 1 || !strings.Contains(fixed, test.want) {
				t.Fatalf("safe fix = (%q, %d, %v), want to preserve %q", fixed, count, err, test.want)
			}
			for _, remaining := range AnalyzeDocument(ass.Parse(fixed)) {
				if remaining.ID == IssueNoEffect {
					t.Fatalf("fixed dialogue still has ASS006: %#v", remaining)
				}
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

func parseStyleOverrideText(format, style, text string) ass.Document {
	return ass.Parse("[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: " + format + "\nStyle: " + style + "\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Text\n" +
		"Dialogue: 0, 0:00:00.00, 0:00:02.00, Default," + text + "\n")
}

func fontOverrideDocument(text, extraStyle string) string {
	return "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, Bold, Italic, ScaleX, ScaleY, Spacing\n" +
		"Style: Default, Arial, 20, 0, 0, 100, 100, 0\n" + extraStyle +
		"[Events]\nFormat: Layer, Start, End, Style, Text\n" +
		"Dialogue: 0, 0:00:00.00, 0:00:02.00, Default," + text + "\n"
}
