package lint

import (
	"assx/internal/ass/renderer"
	"testing"
)

func TestMalformedDrawingRule(t *testing.T) {
	doc := renderer.Parse("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\p1}m 0 0 l 10{\\p0}\n")
	diagnostics := AnalyzeDocument(doc)
	var found *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].Condition == IssueMalformedDrawing {
			found = &diagnostics[i]
			break
		}
	}
	if found == nil || found.Severity != Warning || found.FixSafety != "" || len(found.Edits) != 0 {
		t.Fatalf("drawing = %#v", found)
	}
}
