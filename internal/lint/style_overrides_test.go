package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestRedundantStyleOverridesSafeFix(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "redundant-font.ass")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	doc := ass.Parse(string(data))
	diagnostics := AnalyzeDocument(doc)
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantStyleOverrides {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].FixSafety != SafeFix || len(diagnostics[0].Edits) != 3 {
		t.Fatalf("redundant style finding = %#v", diagnostics[0])
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	fixedDoc := ass.Parse(fixed)
	if count != 1 || len(fixedDoc.Dialogues) != 1 || fixedDoc.Dialogues[0].Text != "alpha beta" {
		t.Fatalf("fixed document (%d edits):\n%s", count, fixed)
	}
	if remaining := AnalyzeRedundantStyleOverrides(ass.Parse(fixed)); len(remaining) != 0 {
		t.Fatalf("fixed document still has redundant style overrides: %#v", remaining)
	}
}

func TestRedundantStyleOverrideStateTracking(t *testing.T) {
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
		{name: "Style-backed state differs at first text", text: `{\fnCourier New}A{\fnArial}B`},
		{name: "safe prefix before state change", text: `{\fnArial}A{\fnCourier New}B{\fnArial}C`, want: true},
		{name: "safe prefix before unresolved style reset", text: `{\fnArial}A{\rOther}B`, want: true},
		{name: "safe prefix before transform", text: `{\fnArial}A{\t(0,500,\fs24)}B`, want: true},
		{name: "rotation has no Style field", text: `{\frz0}A`},
		{name: "encoding has no Style field", text: `{\fe1}A`},
		{name: "font family comparison is exact", text: `{\fnarial}A`},
		{name: "weight values are restricted", text: `{\b100}A`},
		{name: "invalid numeric values are skipped", text: `{\fs20px}A`},
		{name: "no visible text", text: `{\fnArial}`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := parseFontOverrideText(test.text)
			got := len(AnalyzeRedundantStyleOverrides(doc)) != 0
			if got != test.want {
				t.Fatalf("found redundant style override = %t, want %t", got, test.want)
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
			diagnostics := AnalyzeRedundantStyleOverrides(doc)
			if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantStyleOverrides {
				t.Fatalf("diagnostics = %#v", diagnostics)
			}
			if diagnostics[0].Severity != Suggestion || diagnostics[0].FixSafety != SafeFix {
				t.Fatalf("finding metadata = %#v", diagnostics[0])
			}
			if rule := Rules[IssueRedundantStyleOverrides]; rule.Title != "Overrides match style" || rule.Severity != Suggestion || rule.FixSafety != SafeFix {
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
	if diagnostics := AnalyzeRedundantStyleOverrides(mixedAlpha); len(diagnostics) != 0 {
		t.Fatalf("partially matching alpha tag was reported: %#v", diagnostics)
	}

	doc := parseStyleOverrideText(format, style, `{\bord4\bord2}A`)
	diagnostics := AnalyzeDocument(doc)
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantStyleOverrides {
		t.Fatalf("redundant sequence diagnostics = %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil || count != 1 || !strings.Contains(fixed, "Default,,0,0,0,,A\n") {
		t.Fatalf("sequence fix = (%q, %d, %v)", fixed, count, err)
	}

	overlap := AnalyzeDocument(parseStyleOverrideText(format, style, `{\bord2\bord2}A`))
	if len(overlap) != 1 || overlap[0].ID != IssueRedundantStyleOverrides {
		t.Fatalf("ASS006/ASS013 overlap was not assigned to ASS013: %#v", overlap)
	}
}

func TestRedundantStyleOverridesDoNotModelUnsupportedProperties(t *testing.T) {
	tags := []string{`{\blur1}A`, `{\be1}A`, `{\frx0}A`, `{\fry0}A`, `{\fax0}A`, `{\fay0}A`, `{\clip(0,0,1,1)}A`, `{\p1}A`, `{\pbo0}A`}
	for _, text := range tags {
		t.Run(text, func(t *testing.T) {
			doc := parseStyleOverrideText("Name, Outline", "Default,2", text)
			if diagnostics := AnalyzeRedundantStyleOverrides(doc); len(diagnostics) != 0 {
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
			text: `{\fs20\fs20}A`, want: `Default,,0,0,0,,{\fs20}A`,
		},
		{
			name: "multi-slot border", format: "Name, Outline", style: "Default,3",
			text: `{\xbord2\ybord2\bord2}A`, want: `Default,,0,0,0,,{\xbord2\ybord2}A`,
		},
		{
			name: "overwritten before use", format: "Name, Fontsize", style: "Default,30",
			text: `{\fs10\fs20}A`, want: `Default,,0,0,0,,{\fs20}A`,
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

func TestRedundantStyleFixPreservesOtherTags(t *testing.T) {
	doc := parseFontOverrideText(`{\c&H00FF00&\fnCourier New\fnArial}A{\fnArial}B`)
	diagnostics := AnalyzeDocument(doc)
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantStyleOverrides {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || !strings.Contains(fixed, `Dialogue: 0, 0:00:00.00, 0:00:02.00, Default,,0,0,0,,{\c&H00FF00&}AB`) {
		t.Fatalf("other formatting tag was not preserved (%d fixes):\n%s", count, fixed)
	}
}

func TestRedundantStyleRuleRejectsDuplicateDefinitions(t *testing.T) {
	text := fontOverrideDocument(`{\fnArial}A`, "")
	text = strings.Replace(text, "Style: Default, Arial, 20, 0, 0, 100, 100, 0\n", "Style: Default, Arial, 20, 0, 0, 100, 100, 0\nStyle: Default, Arial, 20, 0, 0, 100, 100, 0\n", 1)
	if diagnostics := AnalyzeRedundantStyleOverrides(ass.Parse(text)); len(diagnostics) != 0 {
		t.Fatalf("ambiguous style produced findings: %#v", diagnostics)
	}
}

func TestRedundantStyleRuleUsesRendererDialogueStyleLookup(t *testing.T) {
	definitions := "Style: Default,Arial,20\n"
	for _, name := range []string{"Default", "default", "DEFAULT", "*Default", "  *DEFAULT  "} {
		t.Run(name, func(t *testing.T) {
			doc := parseStyleDefinitionsText("Name, Fontname, Fontsize", definitions, name, `{\fs20}A`)
			var styleFinding, undefinedFinding bool
			for _, diagnostic := range AnalyzeDocument(doc) {
				styleFinding = styleFinding || diagnostic.ID == IssueRedundantStyleOverrides
				undefinedFinding = undefinedFinding || diagnostic.ID == IssueUndefinedStyle
			}
			if !styleFinding || undefinedFinding {
				t.Fatalf("dialogue Style %q did not resolve consistently: style=%t undefined=%t", name, styleFinding, undefinedFinding)
			}
		})
	}
}

func TestRedundantStyleRuleKeepsNonDefaultNamesCaseSensitive(t *testing.T) {
	definitions := "Style: Main,Arial,20\nStyle: main,Arial,30\n"
	for _, test := range []struct {
		name string
		text string
		want bool
	}{
		{name: "Main", text: `{\fs20}A`, want: true},
		{name: "main", text: `{\fs30}A`, want: true},
		{name: "MAIN", text: `{\fs20}A`},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := parseStyleDefinitionsText("Name, Fontname, Fontsize", definitions, test.name, test.text)
			var styleFinding, undefinedFinding bool
			for _, diagnostic := range AnalyzeDocument(doc) {
				styleFinding = styleFinding || diagnostic.ID == IssueRedundantStyleOverrides
				undefinedFinding = undefinedFinding || diagnostic.ID == IssueUndefinedStyle
			}
			if styleFinding != test.want || undefinedFinding == test.want {
				t.Fatalf("Style %q: redundant=%t undefined=%t, want redundant=%t", test.name, styleFinding, undefinedFinding, test.want)
			}
		})
	}
}

func parseFontOverrideText(text string) ass.Document {
	return ass.Parse(fontOverrideDocument(text, ""))
}

func parseStyleOverrideText(format, style, text string) ass.Document {
	return parseStyleDefinitionsText(format, "Style: "+style+"\n", "Default", text)
}

func parseStyleDefinitionsText(format, definitions, dialogueStyle, text string) ass.Document {
	return ass.Parse("[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: " + format + "\n" + definitions +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0, 0:00:00.00, 0:00:02.00, " + dialogueStyle + ",,0,0,0,," + text + "\n")
}

func fontOverrideDocument(text, extraStyle string) string {
	return "[Script Info]\nPlayResX: 640\nPlayResY: 480\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, Bold, Italic, ScaleX, ScaleY, Spacing\n" +
		"Style: Default, Arial, 20, 0, 0, 100, 100, 0\n" + extraStyle +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0, 0:00:00.00, 0:00:02.00, Default,,0,0,0,," + text + "\n"
}

func TestRedundantStyleOverridesFollowActiveResetStyle(t *testing.T) {
	doc := parseStyleDefinitionsText(
		"Name, Fontname, Fontsize",
		"Style: Default,Arial,20\nStyle: Other,Courier New,24\n",
		"Default",
		`{\rOther\fs24}A{\r\fs20}B`,
	)
	diagnostics := AnalyzeRedundantStyleOverrides(doc)
	if len(diagnostics) != 1 || diagnostics[0].ID != IssueRedundantStyleOverrides || len(diagnostics[0].Edits) != 2 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("fix count = %d", count)
	}
	fixedDoc := ass.Parse(fixed)
	if len(fixedDoc.Dialogues) != 1 || fixedDoc.Dialogues[0].Text != `{\rOther}A{\r}B` {
		t.Fatalf("fixed dialogue = %#v", fixedDoc.Dialogues)
	}
}
