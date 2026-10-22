package lint

import (
	"encoding/json"
	"strings"
	"testing"

	"assx/internal/ass"
)

func TestDocumentDiagnosticsCarryExactDecodedSourceSpans(t *testing.T) {
	source := "[Script Info]\nPlayResX: 640\n[V4+ Styles]\nFormat: Name, Alignment\nStyle: Default, 10\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,bad,0:00:01.00,Default,,0,0,0,,{\\unknown}text\n"
	diagnostics := AnalyzeDocument(ass.Parse(source))
	for _, tc := range []struct{ id, literal string }{
		{IssueStyleAlignment, "10"},
		{IssueTimecode, "bad"},
		{IssueUnknownTag, `\unknown`},
	} {
		found := false
		for _, diagnostic := range diagnostics {
			if diagnostic.ID != tc.id {
				continue
			}
			if diagnostic.SourceSpan == nil || diagnostic.SourceSpan.Start < 0 || diagnostic.SourceSpan.End > len(source) || source[diagnostic.SourceSpan.Start:diagnostic.SourceSpan.End] != tc.literal {
				t.Fatalf("%s span = %+v, want %q", tc.id, diagnostic.SourceSpan, tc.literal)
			}
			found = true
			break
		}
		if !found {
			t.Fatalf("missing %s in %v", tc.id, diagnostics)
		}
	}
}

func TestIgnoredTailSpanSelectsOnlyIgnoredBytes(t *testing.T) {
	source := "[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\fs20junk}x\n"
	for _, d := range AnalyzeDocument(ass.Parse(source)) {
		if d.ID == IssueOverrideJunk {
			if d.SourceSpan == nil || source[d.SourceSpan.Start:d.SourceSpan.End] != "junk" {
				t.Fatalf("ignored tail span = %+v", d.SourceSpan)
			}
			return
		}
	}
	t.Fatal("missing ignored-tail diagnostic")
}

func TestCompatibilitySpanSurvivesJSONRoundTrip(t *testing.T) {
	source := "[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\blur101}x\n"
	for _, d := range AnalyzeDocument(ass.Parse(source)) {
		if d.SourceSpan == nil || !strings.Contains(d.Renderer, " / ") {
			continue
		}
		first, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		var decoded Diagnostic
		if err := json.Unmarshal(first, &decoded); err != nil {
			t.Fatal(err)
		}
		second, err := json.Marshal(decoded)
		if err != nil || string(first) != string(second) {
			t.Fatalf("compatibility JSON changed: %s to %s, %v", first, second, err)
		}
		return
	}
	t.Fatal("missing compatibility diagnostic")
}

func TestMissingHeaderUsesAnInsertionPoint(t *testing.T) {
	source := "[Events]\nDialogue: broken\n"
	for _, diagnostic := range AnalyzeDocument(ass.Parse(source)) {
		if diagnostic.ID != IssueMatrixHeader {
			continue
		}
		if diagnostic.SourceSpan == nil || diagnostic.SourceSpan.Start != diagnostic.SourceSpan.End || diagnostic.SourceSpan.Start > len(source) {
			t.Fatalf("missing header span = %+v", diagnostic.SourceSpan)
		}
		return
	}
	t.Fatal("missing matrix diagnostic: " + strings.TrimSpace(source))
}

func TestSyntaxAndReferenceSpansUseSourceTokens(t *testing.T) {
	source := "[Script Info]\nplayresx: 640\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Missing,,0,0,0,,{\\rAbsent}x\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\fs20\n"
	for _, tc := range []struct{ id, literal string }{
		{IssueKeywordCase, "playresx:"},
		{IssueUndefinedStyle, "Missing"},
		{IssueUnterminatedBlock, "{"},
	} {
		found := false
		for _, d := range AnalyzeDocument(ass.Parse(source)) {
			if d.ID == tc.id && d.SourceSpan != nil && source[d.SourceSpan.Start:d.SourceSpan.End] == tc.literal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing %s span for %q", tc.id, tc.literal)
		}
	}
}

func TestNestedOverrideAndResetSpans(t *testing.T) {
	source := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\t(0,100,\\pos(1,2,3))\\rAbsent}x\n"
	diagnostics := AnalyzeDocument(ass.Parse(source))
	for _, literal := range []string{`\pos(1,2,3)`, "Absent"} {
		found := false
		for _, diagnostic := range diagnostics {
			if diagnostic.SourceSpan != nil && source[diagnostic.SourceSpan.Start:diagnostic.SourceSpan.End] == literal {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no source span for %q", literal)
		}
	}
}

func TestUndefinedStyleSpanExcludesFieldWhitespace(t *testing.T) {
	source := "[Events]\nDialogue: 0,0:00:00.00,0:00:01.00,  Missing  ,,0,0,0,,text\n"
	for _, d := range AnalyzeDocument(ass.Parse(source)) {
		if d.ID == IssueUndefinedStyle {
			if d.SourceSpan == nil || source[d.SourceSpan.Start:d.SourceSpan.End] != "Missing" {
				t.Fatalf("undefined style span = %+v", d.SourceSpan)
			}
			return
		}
	}
	t.Fatal("missing undefined style diagnostic")
}

func TestDocumentFindingsHaveBoundedSourceSpans(t *testing.T) {
	source := "[Script Info]\nPlayResX: 640\n[V4+ Styles]\nFormat: Name, Alignment\nStyle: Default, 10\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,bad,0:00:01.00,Missing,,0,0,0,unknown,{\\fs20junk\\t(0,100,\\pos(1,2,3))}x\n"
	for _, d := range AnalyzeDocument(ass.Parse(source)) {
		if d.SourceSpan == nil || d.SourceSpan.Start < 0 || d.SourceSpan.End < d.SourceSpan.Start || d.SourceSpan.End > len(source) {
			t.Errorf("%s has invalid span %+v", d.ID, d.SourceSpan)
		}
	}
}
