package lint

import (
	"fmt"

	"assx/internal/edit"
)

func CountFixes(diagnostics []Diagnostic) (safe, unsafe, unfixable int) {
	for _, diagnostic := range diagnostics {
		if len(diagnostic.Edits) == 0 {
			unfixable++
			continue
		}
		switch diagnostic.FixSafety {
		case SafeFix:
			if diagnostic.FixProof != nil {
				safe++
			} else {
				unfixable++
			}
		case UnsafeFix:
			unsafe++
		default:
			unfixable++
		}
	}
	return safe, unsafe, unfixable
}

func ApplyFixes(text string, diagnostics []Diagnostic, includeUnsafe bool) (string, int, error) {
	var edits []edit.TextEdit
	var safeEdits []FixEditProof
	var proof *FixProof
	fixes := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety == SafeFix {
			if len(diagnostic.Edits) == 0 || diagnostic.FixProof == nil {
				continue
			}
			if proof == nil {
				proof = diagnostic.FixProof
			} else if proof.SourceSHA256 != diagnostic.FixProof.SourceSHA256 ||
				!sameFixTargets(proof.Targets, diagnostic.FixProof.Targets) {
				return "", 0, fmt.Errorf("SafeFix diagnostics use different proof scopes")
			}
			for _, sourceEdit := range diagnostic.Edits {
				item := FixEditProof{ID: diagnostic.ID, Edit: sourceEdit}
				if !proofContainsEdit(proof, item) {
					return "", 0, fmt.Errorf("SafeFix edit is outside its proof scope")
				}
				safeEdits = append(safeEdits, item)
			}
			edits = append(edits, diagnostic.Edits...)
			fixes++
			continue
		}
		if !includeUnsafe || diagnostic.FixSafety != UnsafeFix || len(diagnostic.Edits) == 0 {
			continue
		}
		edits = append(edits, diagnostic.Edits...)
		fixes++
	}
	if len(safeEdits) != 0 {
		if err := verifyFixProofs(text, safeEdits, proof); err != nil {
			return "", 0, fmt.Errorf("verify SafeFix edits: %w", err)
		}
	}
	if len(edits) == 0 {
		return text, 0, nil
	}

	fixed, err := edit.Apply(text, edits)
	if err != nil {
		return "", 0, err
	}
	return fixed, fixes, nil
}

func proofContainsEdit(proof *FixProof, candidate FixEditProof) bool {
	for _, sourceEdit := range proof.SourceEdits {
		if sourceEdit == candidate {
			return true
		}
	}
	return false
}
