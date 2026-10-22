package lintreport

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
)

func TestMissingHeaderKeepsCLICaretOnReportedLine(t *testing.T) {
	doc := ass.Parse("[Script Info]\n[Events]\nDialogue: broken\n")
	view := Build("test.ass", lint.AnalyzeDocument(doc), doc)
	for _, diagnostic := range view.Groups[0].Diagnostics {
		if diagnostic.ID == lint.IssueMatrixHeader {
			if diagnostic.SourceColumn != 1 {
				t.Fatalf("missing header CLI column = %d, want 1", diagnostic.SourceColumn)
			}
			return
		}
	}
	t.Fatal("missing matrix diagnostic")
}

func TestShortEventKeepsCLICaretAtReportedColumn(t *testing.T) {
	doc := ass.Parse("[Events]\nFormat: Layer, Start, End, Text\nDialogue: 1234\n")
	findings := lint.AnalyzeDocument(doc)
	view := Build("test.ass", findings, doc)
	for i, d := range findings {
		if d.ID == lint.IssueShortEvent {
			shown := view.Groups[0].Diagnostics[i]
			if shown.SourceColumn != d.Column || shown.SourceWidth != 1 {
				t.Fatalf("short event CLI caret = %d:%d, want %d:1", shown.SourceColumn, shown.SourceWidth, d.Column)
			}
			return
		}
	}
	t.Fatal("missing short-event diagnostic")
}
