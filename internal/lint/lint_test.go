package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestLexMarksTransitionTagsAndSourceSpans(t *testing.T) {
	text := `pre{\blur6\t(0,650,0.1,\blur0.6)}post`
	tokens := Lex(text)
	var tags []Tag
	for _, token := range tokens {
		if token.Tag != nil {
			tags = append(tags, *token.Tag)
		}
	}
	if len(tags) != 3 {
		t.Fatalf("got %d tags, want 3: %#v", len(tags), tags)
	}
	if tags[0].Name != "blur" || tags[0].InTransition {
		t.Fatalf("outer tag = %#v", tags[0])
	}
	if tags[2].Name != "blur" || !tags[2].InTransition {
		t.Fatalf("nested tag = %#v", tags[2])
	}
	if got := text[tags[2].Start:tags[2].End]; got != `\blur0.6` {
		t.Fatalf("nested source span = %q", got)
	}
}

func TestLexFindsExcessBackslashesOnlyInsideOverrideBlocks(t *testing.T) {
	tokens := Lex(`{\\blur2\mystery3} outside \\text`)
	if len(tokens) != 4 {
		t.Fatalf("got %d tokens: %#v", len(tokens), tokens)
	}
	if tokens[0].Tag == nil || tokens[0].Tag.RepeatedSlashes != 1 {
		t.Fatalf("first token = %#v, want repeated slash marker", tokens[0])
	}
	if tokens[1].Tag == nil || tokens[1].Tag.Name != "blur" {
		t.Fatalf("second token = %#v, want blur tag", tokens[1])
	}
	if tokens[2].Tag == nil || tokens[2].Tag.Name != "mystery3" {
		t.Fatalf("third token = %#v, want unknown tag", tokens[2])
	}
	if tokens[3].Tag != nil || tokens[3].Text != " outside \\\\text" {
		t.Fatalf("outside text token = %#v", tokens[3])
	}
	triple := Lex(`{\\\blur1}`)
	if len(triple) != 2 || triple[0].Tag == nil || triple[0].Tag.RepeatedSlashes != 2 || triple[1].Tag == nil || triple[1].Tag.Name != "blur" {
		t.Fatalf("three-slash tokenization = %#v", triple)
	}
}

func TestAnalyzeReportsUnknownAndRepeatedSlashAsLint(t *testing.T) {
	diagnostics := Analyze(ass.Dialogue{Text: `{\\blur2\mystery}`})
	ids := make(map[string]Severity)
	for _, diagnostic := range diagnostics {
		ids[diagnostic.ID] = diagnostic.Severity
	}
	if ids[IssueRepeatedSlash] != Lint || ids[IssueUnknownTag] != Lint {
		t.Fatalf("diagnostic severities = %#v", ids)
	}
}

func TestAnalyzeAssignFirstWinsAndTransitionBehavior(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{name: "assigned override", text: `{\fs10\fs20}x`, want: 1},
		{name: "first wins", text: `{\an5\an1}x`, want: 1},
		{name: "transition retains start value", text: `{\blur6\t(0,650,0.1,\blur0.6)}x`, want: 0},
		{name: "reset before text", text: `{\fs10\r}x`, want: 1},
		{name: "overridden after text remains effective", text: `{\fs10}x{\fs20}y`, want: 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			count := 0
			for _, diagnostic := range Analyze(ass.Dialogue{Text: test.text}) {
				if diagnostic.ID == IssueNoEffect {
					count++
				}
			}
			if count != test.want {
				t.Fatalf("got %d no-effect findings, want %d", count, test.want)
			}
		})
	}
}

func TestSafeFixRemovesOnlyTheRedundantTag(t *testing.T) {
	text := `{\fs10\fs20}word`
	diagnostics := Analyze(ass.Dialogue{Text: text})
	fixed, count, err := ApplyFixes(text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 || fixed != `{\fs20}word` {
		t.Fatalf("safe fix = (%q, %d), want (%q, 1)", fixed, count, `{\fs20}word`)
	}
}

func TestHeaderChecksAndUnsafeFixes(t *testing.T) {
	text := "[Script Info]\r\nPlayResX: 1920\r\nPlayResY: 1080\r\nYCbCr Matrix: TV.601\r\n[Events]\r\nFormat: Layer, Start, End, Text\r\nDialogue: 0,0,1,text\r\n"
	doc := ass.Parse(text)
	diagnostics := AnalyzeHeaders(doc)
	if len(diagnostics) != 2 || diagnostics[0].ID != IssueMatrixHeader || diagnostics[1].ID != IssueLayoutRes {
		t.Fatalf("header diagnostics = %#v", diagnostics)
	}
	if diagnostics[0].FixSafety != UnsafeFix || diagnostics[1].FixSafety != UnsafeFix {
		t.Fatalf("header fix safety = %#v", diagnostics)
	}
	unchanged, count, err := ApplyFixes(text, diagnostics, false)
	if err != nil || unchanged != text || count != 0 {
		t.Fatalf("safe-only apply = (%q, %d, %v)", unchanged, count, err)
	}
	fixed, count, err := ApplyFixes(text, diagnostics, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !strings.Contains(fixed, "YCbCr Matrix: TV.709") || !strings.Contains(fixed, "LayoutResX: 1920\r\nLayoutResY: 1080\r\n") {
		t.Fatalf("unsafe fixed text (%d fixes):\n%s", count, fixed)
	}
	if got := len(AnalyzeHeaders(ass.Parse(fixed))); got != 0 {
		t.Fatalf("fixed headers still produce %d diagnostics", got)
	}
}

func TestMissingMatrixUnsafeFixAddsExplicitNone(t *testing.T) {
	text := "[Script Info]\nPlayResX: 640\nPlayResY: 480\n[Events]\n"
	diagnostics := AnalyzeHeaders(ass.Parse(text))
	fixed, count, err := ApplyFixes(text, diagnostics, true)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 || !strings.Contains(fixed, "[Script Info]\nYCbCr Matrix: None\nLayoutResX: 640\nLayoutResY: 480\nPlayResX: 640") {
		t.Fatalf("missing header fixes = (%d, %q)", count, fixed)
	}
}

func TestApplyFixesRejectsOverlappingEdits(t *testing.T) {
	diagnostics := []Diagnostic{{FixSafety: SafeFix, Edits: []TextEdit{{Start: 1, End: 3}, {Start: 2, End: 4}}}}
	if _, _, err := ApplyFixes("abcdef", diagnostics, false); err == nil {
		t.Fatal("expected overlapping edit error")
	}
}

func TestRuleRegistryHasStableMetadata(t *testing.T) {
	for _, id := range []string{IssueArgumentCount, IssueInvalidValue, IssueRendererDiff, IssueFontComma, IssueVSFilterModTag, IssueNoEffect, IssueUnknownTag, IssueMatrixHeader, IssueLayoutRes, IssueRepeatedSlash, IssueStyleInteger, IssueStyleFloat, IssueRedundantFontOverrides} {
		rule, ok := Rules[id]
		if !ok || rule.ID != id || rule.Title == "" || rule.Description == "" {
			t.Errorf("rule %q is missing required metadata: %#v", id, rule)
		}
	}
}
