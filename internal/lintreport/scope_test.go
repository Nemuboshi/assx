package lintreport

import (
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
)

func TestCompatibilityScopeUsesPhysicalColumn(t *testing.T) {
	source := "[Script Info]\nScriptType: v4.00+\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\blur101}x\n"
	doc := ass.Parse(source)
	findings := lint.AnalyzeDocument(doc)
	view := Build("scope.ass", findings, doc)
	found := false
	for i, diagnostic := range findings {
		if !strings.Contains(diagnostic.Renderer, " / ") || diagnostic.Tag != "blur" {
			continue
		}
		found = true
		shown := view.Groups[0].Diagnostics[i]
		if shown.SourceColumn != diagnostic.Column || shown.Renderer != diagnostic.Renderer {
			t.Fatalf("compatibility span or scope changed: column %d to %d", diagnostic.Column, shown.SourceColumn)
		}
	}
	if !found {
		t.Fatal("missing compatibility observation")
	}
}

func TestStandaloneCompatibilitySpanMapsToDocument(t *testing.T) {
	const prefix = "Dialogue: "
	text := `{\blur101}x`
	dialogue := ass.Dialogue{Text: text, Line: 1, TextStart: len(prefix)}
	doc := ass.Document{Text: prefix + text, Dialogues: []ass.Dialogue{dialogue}}
	findings := lint.AnalyzeCompatibility(dialogue, nil)
	view := Build("scope.ass", findings, doc)
	if len(findings) == 0 {
		t.Fatal("missing compatibility findings")
	}
	for i, d := range findings {
		shown := view.Groups[0].Diagnostics[i]
		if shown.SourceColumn != len(prefix)+2 || shown.SourceWidth != len(`\blur101`) || d.SourceSpan == nil {
			t.Fatalf("standalone span mislocated: %d:%d", shown.SourceColumn, shown.SourceWidth)
		}
	}
}

func TestPrimaryScopeUsesConcretePrefixSpan(t *testing.T) {
	text := `{\fsvp6}x`
	const prefix = "Dialogue: "
	doc := ass.Document{Text: prefix + text, Dialogues: []ass.Dialogue{{Text: text, Line: 1, TextStart: len(prefix)}}}
	findings := lint.Analyze(doc.Dialogues[0])
	view := Build("scope.ass", findings, doc)
	found := false
	for i, diagnostic := range findings {
		if diagnostic.ID != lint.IssueInvalidValue || diagnostic.Tag != "fs" {
			continue
		}
		found = true
		shown := view.Groups[0].Diagnostics[i]
		if shown.SourceColumn != len(prefix)+2 || shown.SourceWidth != len(`\fsvp6`) {
			t.Fatalf("matched prefix lost concrete span: %d:%d", shown.SourceColumn, shown.SourceWidth)
		}
	}
	if !found {
		t.Fatal("missing primary prefix diagnostic")
	}
}
