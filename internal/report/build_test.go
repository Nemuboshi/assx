package report

import (
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
)

func TestBuildFallsBackToPointForMalformedOverrideBlock(t *testing.T) {
	source := `{\pos(1,2)`
	dialogue := ass.Dialogue{Text: source, Line: 1, LineStart: 0, TextStart: 0, Syntax: ass.ParseDialogueText(source)}
	got := Build("sample.ass", []lint.Diagnostic{{ID: "ASS002", Line: 1, Column: 2, Tag: "pos"}}, ass.Document{Text: source, Dialogues: []ass.Dialogue{dialogue}})
	if p := got.Groups[0].Diagnostics[0]; p.SourceColumn != 2 || p.SourceWidth != 1 {
		t.Fatalf("malformed block source = %d:%d, want point at 2", p.SourceColumn, p.SourceWidth)
	}
}

func TestBuildUsesNestedTagSpanAndDoesNotBorrowEditRange(t *testing.T) {
	text := `Dialogue: {\t(0,100,\fs20)}`
	dialogueText := `{\t(0,100,\fs20)}`
	dialogue := ass.Dialogue{Text: dialogueText, Line: 1, LineStart: 0, TextStart: len("Dialogue: "), Syntax: ass.ParseDialogueText(dialogueText)}
	var nestedColumn int
	dialogue.Syntax.WalkTokens(func(token ass.TokenView) bool {
		if token.HasTag && token.Tag.Name == "fs" {
			nestedColumn = token.Tag.Column
			return false
		}
		return true
	})
	doc := ass.Document{Text: text, Dialogues: []ass.Dialogue{dialogue}}
	got := Build("sample.ass", []lint.Diagnostic{
		{ID: "nested", Line: 1, Column: nestedColumn, Tag: "fs"},
		{ID: "unknown", Line: 1, Column: 1, Field: "missing", FixSafety: lint.SafeFix, Edits: []lint.TextEdit{{Start: 1, End: 9}}},
	}, doc)
	if p := got.Groups[0].Diagnostics[0]; p.SourceColumn != len("Dialogue: ")+strings.Index(dialogueText, `\fs20`)+1 || p.SourceWidth != len(`\fs20`) {
		t.Fatalf("nested tag source = %d:%d", p.SourceColumn, p.SourceWidth)
	}
	if p := got.Groups[0].Diagnostics[1]; p.SourceWidth != 1 {
		t.Fatalf("unknown field borrowed edit width: %d", p.SourceWidth)
	}
}
func TestBuildUsesDocumentTextStartAndConcreteFixMetadata(t *testing.T) {
	doc := ass.Document{Text: "12345678901234567890{\\fs20}\nDialogue: ...........label", Dialogues: []ass.Dialogue{
		{Text: "{\\fs20}", Line: 1, LineStart: 0, TextStart: 20, Syntax: ass.ParseDialogueText("{\\fs20}")},
		{Line: 2, LineStart: 28, Fields: []ass.EventField{{Name: "style", Start: 49, End: 54}}},
	}}
	got := Build("sample.ass", []lint.Diagnostic{
		{ID: "safe", Line: 1, Column: 3, Tag: "fs", FixSafety: lint.SafeFix, FixProof: &lint.FixProof{}, Edits: []lint.TextEdit{{Start: 1, End: 2}}},
		{ID: "suggestion", Line: 1, Column: 3, Tag: "bord", Fix: "change it"},
		{ID: "field", Line: 2, Column: 12, Field: "Style"},
	}, doc)
	if got.Groups[0].Diagnostics[0].SourceColumn != 22 {
		t.Fatalf("source column = %d, want 22", got.Groups[0].Diagnostics[0].SourceColumn)
	}
	if got.Groups[0].Diagnostics[0].SourceWidth != 5 {
		t.Fatalf("source width = %d, want 5", got.Groups[0].Diagnostics[0].SourceWidth)
	}
	if got.Groups[0].Diagnostics[2].SourceColumn != 22 || got.Groups[0].Diagnostics[2].SourceWidth != 5 {
		t.Fatalf("field source = %d:%d, want 22:5", got.Groups[0].Diagnostics[2].SourceColumn, got.Groups[0].Diagnostics[2].SourceWidth)
	}
	if got.Groups[0].Diagnostics[0].Outcome != FixSafe || got.Groups[0].Diagnostics[1].Outcome != FixNone {
		t.Fatalf("fix outcomes = %v, %v", got.Groups[0].Diagnostics[0].Outcome, got.Groups[0].Diagnostics[1].Outcome)
	}
}
