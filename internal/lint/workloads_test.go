package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/perftest"
)

const styleEditBlock = `{` + `\b1\b1\fs20\fs20\t(0,500,\fscx100)}`

// styleEditWorkload asserts the Style-edit benchmark contract before timing:
// the fractional MarginL value yields one proved ASS011 edit, every dialogue
// yields proved ASS006 edits, and fixed bytes carry the integer Style value
// and no override-block candidate.
func TestStyleEditWorkloadProvesDocumentWide(t *testing.T) {
	text := perftest.StyleEdit(t)
	doc := ass.Parse(text)
	if len(doc.Dialogues) != perftest.StyleEditDialogueCount {
		t.Fatalf("got %d dialogues, want %d", len(doc.Dialogues), perftest.StyleEditDialogueCount)
	}
	findings := AnalyzeDocument(doc)
	primary := primaryFindings(findings)

	var styleCandidates, styleProofs int
	var dialogueCandidates, dialogueProofs int
	for _, diagnostic := range primary {
		switch diagnostic.ID {
		case IssueStyleInteger:
			styleCandidates++
			if diagnostic.FixSafety == SafeFix && (diagnostic.FixProof != nil || diagnostic.FixProofRef != "") {
				styleProofs++
			}
		case IssueNoEffect:
			dialogueCandidates++
			if diagnostic.FixSafety == SafeFix && (diagnostic.FixProof != nil || diagnostic.FixProofRef != "") {
				dialogueProofs++
			}
		}
	}
	if styleCandidates != 1 || styleProofs != 1 {
		t.Fatalf("style findings: candidates=%d proved=%d, want one proved edit", styleCandidates, styleProofs)
	}
	wantDialogueEdits := perftest.StyleEditDialogueCount * 3
	if dialogueCandidates != wantDialogueEdits {
		t.Fatalf("dialogue candidates=%d, want %d", dialogueCandidates, wantDialogueEdits)
	}
	if dialogueProofs != wantDialogueEdits {
		t.Fatalf("proved dialogue candidates=%d, want %d", dialogueProofs, wantDialogueEdits)
	}
	safe, _, _ := CountFixes(findings)
	if safe != styleCandidates+dialogueCandidates {
		t.Fatalf("safe fix count=%d, want %d", safe, styleCandidates+dialogueCandidates)
	}

	fixed, count, err := ApplyFixes(text, findings, false)
	if err != nil {
		t.Fatal(err)
	}
	if count != safe {
		t.Fatalf("applied %d fixes, want %d", count, safe)
	}
	if !strings.Contains(fixed, ",0,0,0,1") || strings.Contains(fixed, ",0.0,0,0,1") {
		t.Fatal("fixed bytes do not carry the integer MarginL value")
	}
	if strings.Contains(fixed, styleEditBlock) {
		t.Fatal("fixed bytes still carry a full override block")
	}
}

// lateFailureWorkload asserts the late proof-failure shape outside timing:
// all earlier dialogues keep proved edits and the final dialogue's barrier
// revokes exactly its three candidates. Larger scaling sizes run inside the
// benchmark stage guards; this test keeps the fast sizes in go test.
func TestLateProofFailureScalesCandidates(t *testing.T) {
	for _, count := range []int{250, 1000} {
		doc := ass.Parse(benchmarkLateProofFailure(count))
		if len(doc.Dialogues) != count {
			t.Fatalf("late failure %d: got %d dialogues", count, len(doc.Dialogues))
		}
		findings := AnalyzeDocument(doc)
		primary := primaryFindings(findings)
		lastLine := doc.Dialogues[len(doc.Dialogues)-1].Line
		var proved, revoked int
		for _, diagnostic := range primary {
			if diagnostic.ID != IssueNoEffect {
				continue
			}
			if diagnostic.FixSafety == SafeFix && (diagnostic.FixProof != nil || diagnostic.FixProofRef != "") {
				proved++
				continue
			}
			if diagnostic.Line == lastLine {
				revoked++
			}
		}
		if revoked != 3 {
			t.Fatalf("late failure %d: revoked candidates=%d, want 3", count, revoked)
		}
		if want := 3*count - revoked; proved != want {
			t.Fatalf("late failure %d: proved candidates=%d, want %d", count, proved, want)
		}
	}
}
