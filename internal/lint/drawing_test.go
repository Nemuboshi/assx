package lint

import (
	"testing"

	"assx/internal/ass"
)

func TestMalformedDrawingRule(t *testing.T) {
	doc := ass.Parse("[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0,1,Default,{\\p1}m 0 0 l 10{\\p0}\n")
	diagnostics := AnalyzeDocument(doc)
	var found *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == IssueMalformedDrawing {
			found = &diagnostics[i]
			break
		}
	}
	if found == nil || found.Severity != Warning || found.FixSafety != "" || len(found.Edits) != 0 {
		t.Fatalf("ASS020 = %#v", found)
	}
}
