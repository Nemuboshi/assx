package lintreport

import (
	"encoding/json"
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
)

func TestReportFixOutcomesUseValidatedGroupedProofs(t *testing.T) {
	doc := ass.Document{Text: `{\fs21\fs21}A{\fs22\fs22}B`, Dialogues: []ass.Dialogue{{Text: `{\fs21\fs21}A{\fs22\fs22}B`, Line: 1}}}
	diagnostics := lint.Analyze(doc.Dialogues[0])
	data, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []lint.Diagnostic
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, ds := range [][]lint.Diagnostic{diagnostics, decoded} {
		safe, unsafe, unfixable := lint.CountFixes(ds)
		view := Build("proof.ass", ds, doc)
		if safe != 2 || view.Remaining.Safe != safe || view.Remaining.Unsafe != unsafe || view.Remaining.Unfixable != unfixable {
			t.Fatalf("presenter totals disagree with verified metadata: %+v", view.Remaining)
		}
	}
	for i := range decoded {
		if decoded[i].FixProof != nil {
			decoded[i].FixProof.EditedSHA256 = "tampered"
		}
	}
	view := Build("proof.ass", decoded, doc)
	if view.Remaining.Safe != 0 {
		t.Fatal("tampered proof retained a safe badge")
	}
	view = Build("proof.ass", []lint.Diagnostic{{FixSafety: lint.SafeFix, Edits: []lint.TextEdit{{Start: 1, End: 2}}, FixProofRef: "dangling"}}, doc)
	if view.Remaining.Safe != 0 {
		t.Fatal("dangling proof reference retained a safe badge")
	}
}
