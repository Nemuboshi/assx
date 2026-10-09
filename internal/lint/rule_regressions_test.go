package lint

import (
	"os"
	"strings"
	"testing"

	"assx/internal/ass"
)

func diagnosticsForIDs(diags []Diagnostic, ids ...string) []Diagnostic {
	var out []Diagnostic
	for _, diagnostic := range diags {
		for _, id := range ids {
			if diagnostic.ID == id {
				out = append(out, diagnostic)
			}
		}
	}
	return out
}

func TestEmptyParenthesizedClipIsIgnored(t *testing.T) {
	text := `before{\clip()}after`
	dialogue := ass.Dialogue{Text: text, Line: 1, Syntax: ass.ParseDialogueText(text)}
	findings := Analyze(dialogue)
	if got := diagnosticsForIDs(findings, IssueArgumentCount, IssueOverrideJunk); len(got) != 0 {
		t.Fatalf("empty clip produced diagnostics: %#v", got)
	}
}

func TestEmptyParenthesizedClipFixture(t *testing.T) {
	data, err := os.ReadFile("../../testdata/regression-empty-clip.ass")
	if err != nil {
		t.Fatal(err)
	}
	findings := AnalyzeDocument(ass.Parse(string(data)))
	if got := diagnosticsForIDs(findings, IssueArgumentCount, IssueOverrideJunk, IssueUnterminatedBlock); len(got) != 0 {
		t.Fatalf("regression fixture diagnostics = %#v", got)
	}
	if got := diagnosticsForIDs(findings, IssueNoEffect); len(got) != 2 {
		t.Fatalf("expected the two removable bord transforms, got %#v", got)
	}
}

func TestLegacyAlignmentAliasRendererDifference(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: `{\a4}A`, want: true},
		{text: `{\a8}A`, want: true},
		{text: `{\a5}A`, want: false},
		{text: `{\a3}A`, want: false},
		{text: `{\an4}A`, want: false},
		{text: `{\an8}A`, want: false},
		{text: `{\a12}A`, want: false},
	}
	for _, test := range cases {
		dialogue := ass.Dialogue{Text: test.text, Line: 1, Syntax: ass.ParseDialogueText(test.text)}
		found := false
		for _, diagnostic := range Analyze(dialogue) {
			if diagnostic.ID == IssueRendererDiff && diagnostic.Tag == "a" {
				found = true
			}
		}
		if found != test.want {
			t.Fatalf("%s: renderer diff=%v, want %v", test.text, found, test.want)
		}
	}
}

func TestUnterminatedOverrideBlock(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{name: "missing close", text: `{\i1 hello`, want: 1},
		{name: "complete block", text: `{\i1}hello`, want: 0},
		{name: "escaped brace is text", text: `\{literal brace`, want: 0},
		{name: "only first run reported", text: `{\i1 a {\b1 b`, want: 1},
		{name: "trailing open brace", text: `hello {`, want: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			dialogue := ass.Dialogue{Text: test.text, Line: 1, Syntax: ass.ParseDialogueText(test.text)}
			findings := diagnosticsForIDs(Analyze(dialogue), IssueUnterminatedBlock)
			if len(findings) != test.want {
				t.Fatalf("findings = %#v, want %d", findings, test.want)
			}
			if test.want == 1 && findings[0].Column != strings.IndexByte(test.text, '{')+1 {
				t.Fatalf("column = %d, want one-based brace offset", findings[0].Column)
			}
			if len(findings) != 0 && findings[0].FixSafety != "" {
				t.Fatalf("unterminated block must have no automatic fix: %#v", findings[0])
			}
		})
	}
}

func TestMalformedStyleColourFields(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "canonical eight digits", value: "&H80FFFFFF&", want: false},
		{name: "six digits", value: "&HFFFFFF&", want: false},
		{name: "decimal", value: "16777215", want: false},
		{name: "negative decimal", value: "-1", want: false},
		{name: "hex prefix without digit", value: "&HFFFFFF", want: false},
		{name: "no prefix no digit", value: "FFFFFF", want: true},
		{name: "bare prefix", value: "&H&", want: true},
		{name: "empty", value: "", want: true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := ass.Parse("[V4+ Styles]\nFormat: Name, PrimaryColour\nStyle: Default," + test.value + "\n")
			findings := diagnosticsForIDs(AnalyzeStyles(doc), IssueMalformedStyleColour)
			if (len(findings) != 0) != test.want {
				t.Fatalf("findings = %#v, want report=%t", findings, test.want)
			}
			for _, finding := range findings {
				if finding.Severity != Error || finding.FixSafety != "" || len(finding.Edits) != 0 {
					t.Fatalf("colour finding must be an error without a fix: %#v", finding)
				}
			}
		})
	}
}

func TestKeywordCaseWarnings(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []string
	}{
		{name: "canonical keywords", text: "[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,hi\n", want: nil},
		{name: "lowercase dialogue", text: "[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,hi\ndialogue: 0,0:00:01.00,0:00:02.00,Default,drop\n", want: []string{"Dialogue"}},
		{name: "lowercase style", text: "[V4+ Styles]\nFormat: Name, Fontname\nstyle: Default,Arial\n", want: []string{"Style"}},
		{name: "uppercase header keyword", text: "[Script Info]\nPLAYRESX: 640\n", want: []string{"PlayResX"}},
		{name: "space before colon is broken in both", text: "[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue : 0,0:00:00.00,0:00:01.00,Default,hi\n", want: nil},
		{name: "unknown keywords stay quiet", text: "[Events]\nMyTag: 1\n", want: nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := ass.Parse(test.text)
			findings := diagnosticsForIDs(AnalyzeKeywords(doc), IssueKeywordCase)
			if len(findings) != len(test.want) {
				t.Fatalf("findings = %#v, want %v", findings, test.want)
			}
			for i, finding := range findings {
				if finding.Field != test.want[i] || finding.FixSafety != "" {
					t.Fatalf("finding %d = %#v, want field %q and no fix", i, finding, test.want[i])
				}
			}
		})
	}
}

func TestRegistryCoversAllShippedRules(t *testing.T) {
	for _, id := range []string{IssueUnterminatedBlock, IssueMalformedStyleColour, IssueKeywordCase} {
		rule := Rules[id]
		if rule.ID != id || rule.Title == "" || rule.Description == "" || rule.Fix == "" {
			t.Fatalf("rule %s metadata = %#v", id, rule)
		}
		for _, source := range rule.Sources {
			if !strings.HasPrefix(source, "https://github.com/") {
				t.Fatalf("rule %s source is not a pinned repository URL: %s", id, source)
			}
		}
	}
}
