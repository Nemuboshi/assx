package lint

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// AnalyzeNoEffectsForRenderer exposes ASS006 observations for one pinned
// renderer without coupling them to name/arity validation. It shares the
// document analyzer's evaluation and never attaches an automatic fix: a
// single-profile observation is not the all-target equivalence proof that a
// SafeFix requires (P08).
func AnalyzeNoEffectsForRenderer(dialogue ass.Dialogue, profile renderer.Profile) []Diagnostic {
	findings, _ := analyzeResolvedDialogue(dialogue, profile, nil, nil, nil)
	var out []Diagnostic
	for _, finding := range findings {
		if finding.ID != IssueNoEffect {
			continue
		}
		// A single-profile observation never carries an edit or a proof.
		finding.Edits = nil
		finding.FixSafety = ""
		finding.FixProof = nil
		finding.FixProofRef = ""
		out = append(out, finding)
	}
	return scopeDiagnostics(out, profile)
}
