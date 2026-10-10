package lint

import (
	"fmt"
	"reflect"

	"assx/internal/edit"
)

func CountFixes(diagnostics []Diagnostic) (safe, unsafe, unfixable int) {
	proofs := fixProofsByID(diagnostics)
	validProofs := make(map[string]bool, len(proofs))
	for id, proof := range proofs {
		validProofs[id] = proofIdentityMatches(proof)
	}
	for _, diagnostic := range diagnostics {
		if len(diagnostic.Edits) == 0 {
			unfixable++
			continue
		}
		switch diagnostic.FixSafety {
		case SafeFix:
			proof := diagnostic.FixProof
			if diagnostic.FixProofRef != "" {
				proof = proofs[diagnostic.FixProofRef]
				if diagnostic.FixProof != nil && !reflect.DeepEqual(diagnostic.FixProof, proof) {
					proof = nil
				}
			}
			if proof != nil && validProofs[proof.ID] {
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

func fixProofsByID(diagnostics []Diagnostic) map[string]*FixProof {
	proofs := make(map[string]*FixProof)
	for i := range diagnostics {
		proof := diagnostics[i].FixProof
		if proof != nil && proof.ID != "" {
			if _, exists := proofs[proof.ID]; !exists {
				proofs[proof.ID] = proof
			}
		}
	}
	return proofs
}

type safeProofEdits struct {
	proof *FixProof
	edits []FixEditProof
}

func ApplyFixes(text string, diagnostics []Diagnostic, includeUnsafe bool) (string, int, error) {
	proofs := fixProofsByID(diagnostics)
	for _, diagnostic := range diagnostics {
		if diagnostic.FixProofRef == "" {
			continue
		}
		proof := proofs[diagnostic.FixProofRef]
		if proof == nil || diagnostic.FixProof != nil && !reflect.DeepEqual(diagnostic.FixProof, proof) {
			return "", 0, fmt.Errorf("dangling SafeFix proof reference %q", diagnostic.FixProofRef)
		}
	}

	var edits []edit.TextEdit
	var safeEdits []FixEditProof
	var groups []safeProofEdits
	groupIndexes := make(map[string]int)
	fixes := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety == SafeFix {
			proof := diagnostic.FixProof
			if diagnostic.FixProofRef != "" {
				proof = proofs[diagnostic.FixProofRef]
			}
			if len(diagnostic.Edits) == 0 || proof == nil {
				continue
			}
			if proof.ID == "" {
				return "", 0, fmt.Errorf("SafeFix proof has no identity")
			}
			groupIndex, exists := groupIndexes[proof.ID]
			if !exists {
				groupIndex = len(groups)
				groupIndexes[proof.ID] = groupIndex
				groups = append(groups, safeProofEdits{proof: proof})
			} else if !reflect.DeepEqual(groups[groupIndex].proof, proof) {
				return "", 0, fmt.Errorf("SafeFix proof ID %q has conflicting evidence", proof.ID)
			}
			for _, sourceEdit := range diagnostic.Edits {
				item := FixEditProof{ID: diagnostic.ID, Edit: sourceEdit}
				if !proofContainsEdit(proof, item) {
					return "", 0, fmt.Errorf("SafeFix edit is outside its proof scope")
				}
				groups[groupIndex].edits = append(groups[groupIndex].edits, item)
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
		doc := proofDocument(text, safeEdits)
		validEdits := fixEditIndex(analyzeDocumentUnproved(doc))
		for _, group := range groups {
			if err := verifyFixProofs(text, doc, group.edits, group.proof, validEdits); err != nil {
				return "", 0, fmt.Errorf("verify SafeFix edits: %w", err)
			}
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
