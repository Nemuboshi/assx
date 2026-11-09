package lint

import (
	"assx/internal/ass/renderer"
	"testing"
)

func TestEmptyOverrideBlockAndOverrideJunk(t *testing.T) {
	doc := defaultDocument("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{}A{   }B{comment}C{\\fs20 junk \\bord2}D\n")
	diagnostics := analyzeDocumentTest(doc)

	var empty, junk int
	for _, diagnostic := range diagnostics {
		switch diagnostic.Condition {
		case IssueEmptyOverrideBlock:
			empty++
			if diagnostic.FixSafety == SafeFix || len(diagnostic.Edits) != 0 || diagnostic.FixProof != nil {
				t.Fatalf("empty-block retained a fix in a dialogue with unresolved commands: %#v", diagnostic)
			}
		case IssueOverrideJunk:
			junk++
			if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
				t.Fatalf("junk unexpectedly has an automatic fix: %#v", diagnostic)
			}
		}
	}
	if empty != 2 || junk != 1 {
		t.Fatalf("empty-block=%d junk=%d diagnostics=%#v", empty, junk, diagnostics)
	}

	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 || fixed != doc.Text {
		t.Fatalf("unresolved dialogue changed: count=%d text=%q", count, fixed)
	}
}

func TestCommentBlockIsNotOverrideJunk(t *testing.T) {
	text := "{a normal comment}text"
	dialogue := renderer.Dialogue{Text: text, Line: 1}
	for _, diagnostic := range Analyze(dialogue) {
		if diagnostic.Condition == IssueOverrideJunk || diagnostic.Condition == IssueEmptyOverrideBlock {
			t.Fatalf("comment produced override syntax finding: %#v", diagnostic)
		}
	}
}
