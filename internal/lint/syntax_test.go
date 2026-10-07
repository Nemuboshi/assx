package lint

import (
	"testing"

	"assx/internal/ass"
)

func TestEmptyOverrideBlockAndOverrideJunk(t *testing.T) {
	doc := ass.Parse("[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0,1,Default,{}A{   }B{comment}C{\\fs20 junk \\bord2}D\n")
	diagnostics := AnalyzeDocument(doc)

	var empty, junk int
	for _, diagnostic := range diagnostics {
		switch diagnostic.ID {
		case IssueEmptyOverrideBlock:
			empty++
			if diagnostic.FixSafety != SafeFix || len(diagnostic.Edits) != 1 {
				t.Fatalf("ASS018 is not SafeFix: %#v", diagnostic)
			}
		case IssueOverrideJunk:
			junk++
			if diagnostic.FixSafety != "" || len(diagnostic.Edits) != 0 {
				t.Fatalf("ASS019 unexpectedly has an automatic fix: %#v", diagnostic)
			}
		}
	}
	if empty != 2 || junk != 1 {
		t.Fatalf("ASS018=%d ASS019=%d diagnostics=%#v", empty, junk, diagnostics)
	}

	fixed, count, err := ApplyFixes(doc.Text, diagnostics, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("safe fix count = %d, want 2", count)
	}
	fixedDoc := ass.Parse(fixed)
	if len(fixedDoc.Dialogues) != 1 || fixedDoc.Dialogues[0].Text != "AB{comment}C{\\fs20 junk \\bord2}D" {
		t.Fatalf("fixed dialogue = %#v", fixedDoc.Dialogues)
	}
}

func TestCommentBlockIsNotOverrideJunk(t *testing.T) {
	text := "{a normal comment}text"
	dialogue := ass.Dialogue{Text: text, Line: 1, Syntax: ass.ParseDialogueText(text)}
	for _, diagnostic := range Analyze(dialogue) {
		if diagnostic.ID == IssueOverrideJunk || diagnostic.ID == IssueEmptyOverrideBlock {
			t.Fatalf("comment produced override syntax finding: %#v", diagnostic)
		}
	}
}
