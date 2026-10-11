package lint

import (
	"strings"
	"testing"

	"assx/internal/ass/renderer"
)

func TestApplyRechecksCombinedIndependentProofGroups(t *testing.T) {
	libass, _ := renderer.Standard(renderer.Libass)
	targets := AnalysisTargets{Primary: libass}
	doc := parseFontOverrideText(`{\fs21\fs21}A{\r\fs22\fs22\pos(1,2)\bord2\t(\bord2)}B`)
	findings := AnalyzeDocumentForTargets(doc, targets)
	var separate []Diagnostic
	for _, d := range findings {
		if d.FixSafety != SafeFix || d.ID != IssueNoEffect || d.Tag != "fs" {
			continue
		}
		edits := make([]FixEditProof, 0, len(d.Edits))
		for _, edit := range d.Edits {
			edits = append(edits, FixEditProof{ID: d.ID, Edit: edit})
		}
		proof, err := buildFixProof(doc.Text, edits, targets)
		if err != nil {
			t.Fatal(err)
		}
		d.FixProof, d.FixProofRef = proof, ""
		separate = append(separate, d)
	}
	if len(separate) != 2 {
		t.Fatalf("need two independent proof groups, got %d", len(separate))
	}
	fixed, count, err := ApplyFixesForTargets(doc.Text, separate, false, targets)
	if err != nil || count != 2 || strings.Contains(fixed, `\fs21\fs21`) || strings.Contains(fixed, `\fs22\fs22`) {
		t.Fatalf("combined reset/transform proof: count=%d err=%v", count, err)
	}
}
