package lint

import (
	"fmt"
	"reflect"
	"sync/atomic"

	"assx/internal/edit"
)

func CountFixes(diagnostics []Diagnostic) (safe, unsafe, unfixable int) {
	return NewFixProofIndex(diagnostics).Count(diagnostics)
}

// FixProofIndex is a reusable view of the SafeFix proofs in one diagnostic
// set. CountFixes rebuilds it per call; presenters and callers that inspect
// many diagnostics build it once.
type FixProofIndex struct {
	proofs      map[int]*FixProof
	memberships map[*FixProof]proofEditMembership
}

// NewFixProofIndex indexes the shared proofs by their reference ordinal and
// records which edits each proof authorizes.
func NewFixProofIndex(diagnostics []Diagnostic) FixProofIndex {
	proofs := fixProofsByRef(diagnostics)
	memberships := make(map[*FixProof]proofEditMembership, len(proofs))
	for _, proof := range proofs {
		if _, exists := memberships[proof]; !exists {
			memberships[proof] = proofEditsOf(proof)
		}
	}
	return FixProofIndex{proofs: proofs, memberships: memberships}
}

// Count reports the fix counts for one diagnostic set under this index.
func (index FixProofIndex) Count(diagnostics []Diagnostic) (safe, unsafe, unfixable int) {
	return countFixes(diagnostics, index.proofs, index.memberships)
}

func countFixes(diagnostics []Diagnostic, proofs map[int]*FixProof, memberships map[*FixProof]proofEditMembership) (safe, unsafe, unfixable int) {
	for _, diagnostic := range diagnostics {
		if len(diagnostic.Edits) == 0 {
			unfixable++
			continue
		}
		switch diagnostic.FixSafety {
		case SafeFix:
			proof := diagnostic.FixProof
			if diagnostic.FixProofRef != 0 {
				proof = proofs[diagnostic.FixProofRef]
				if diagnostic.FixProof != nil && !reflect.DeepEqual(diagnostic.FixProof, proof) {
					proof = nil
				}
			}
			if proof != nil {
				membership := memberships[proof]
				authorized := true
				for _, sourceEdit := range diagnostic.Edits {
					if !membership.contains(FixEditProof{Condition: diagnostic.Condition, Edit: sourceEdit}) {
						authorized = false
						break
					}
				}
				if authorized {
					safe++
				} else {
					unfixable++
				}
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

func fixProofsByRef(diagnostics []Diagnostic) map[int]*FixProof {
	proofs := make(map[int]*FixProof)
	for i := range diagnostics {
		diagnostic := diagnostics[i]
		if diagnostic.FixProof == nil {
			continue
		}
		if _, exists := proofs[diagnostic.FixProofRef]; !exists {
			proofs[diagnostic.FixProofRef] = diagnostic.FixProof
		}
	}
	return proofs
}

type safeProofEdits struct {
	proof *FixProof
	edits []FixEditProof
	// known authorizes this group's candidate edits from the proof's own list,
	// built once per proof instead of scanned once per edit.
	known proofEditMembership
}

// proofMembershipComparisons counts the source-edit membership decisions the
// fix application pass performs: one per candidate edit plus one per proof
// source edit examined while the pass scanned the proof's list, and one per
// candidate edit under the membership index. Tests watch it to prove
// authorization does not rescan a proof's whole edit list for every edit.
var proofMembershipComparisons atomic.Int64

// proofEditMembership indexes one proof's own source edits so one candidate
// edit is authorized in constant time. The key is the complete proof entry, so
// an entry with a different rule identity, source span, or replacement text
// stays outside the proof's scope, and repeated entries authorize the same
// edit.
type proofEditMembership map[FixEditProof]struct{}

func proofEditsOf(proof *FixProof) proofEditMembership {
	membership := make(proofEditMembership, len(proof.SourceEdits))
	for _, sourceEdit := range proof.SourceEdits {
		membership[sourceEdit] = struct{}{}
	}
	return membership
}

func (membership proofEditMembership) contains(candidate FixEditProof) bool {
	proofMembershipComparisons.Add(1)
	_, ok := membership[candidate]
	return ok
}

func ApplyFixes(text string, diagnostics []Diagnostic, includeUnsafe bool) (string, int, error) {
	return ApplyFixesForTargets(text, diagnostics, includeUnsafe, DefaultAnalysisTargets())
}

// ApplyFixesForTargets checks every SafeFix against the same analysis policy.
func ApplyFixesForTargets(text string, diagnostics []Diagnostic, includeUnsafe bool, targets AnalysisTargets) (string, int, error) {
	return applyFixes(text, diagnostics, includeUnsafe, targets, nil)
}

// applyFixes re-authorizes every SafeFix under the given rule policy before
// it writes anything. A nil policy means the default condition catalog.
func applyFixes(text string, diagnostics []Diagnostic, includeUnsafe bool, targets AnalysisTargets, policy RulePolicy) (string, int, error) {
	proofs := fixProofsByRef(diagnostics)
	for _, diagnostic := range diagnostics {
		if diagnostic.FixProofRef == 0 {
			continue
		}
		proof := proofs[diagnostic.FixProofRef]
		if proof == nil || diagnostic.FixProof != nil && !reflect.DeepEqual(diagnostic.FixProof, proof) {
			return "", 0, fmt.Errorf("dangling SafeFix proof reference %d", diagnostic.FixProofRef)
		}
	}

	var edits []edit.TextEdit
	var safeEdits []FixEditProof
	var groups []safeProofEdits
	fixes := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety == SafeFix {
			proof := diagnostic.FixProof
			if diagnostic.FixProofRef != 0 {
				proof = proofs[diagnostic.FixProofRef]
			}
			if len(diagnostic.Edits) == 0 || proof == nil {
				continue
			}
			groupIndex := -1
			for i := range groups {
				if groups[i].proof == proof || reflect.DeepEqual(groups[i].proof, proof) {
					groupIndex = i
					break
				}
			}
			if groupIndex < 0 {
				groupIndex = len(groups)
				groups = append(groups, safeProofEdits{proof: proof, known: proofEditsOf(proof)})
			}
			for _, sourceEdit := range diagnostic.Edits {
				item := FixEditProof{Condition: diagnostic.Condition, Edit: sourceEdit}
				if !groups[groupIndex].known.contains(item) {
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
		validEdits := fixEditIndex(analyzeDocumentForTargetsUnproved(doc, targets, policy))
		for _, group := range groups {
			if err := verifyFixProofs(text, doc, group.edits, group.proof, targets.Profiles(), validEdits); err != nil {
				return "", 0, fmt.Errorf("verify SafeFix edits: %w", err)
			}
		}
		if len(groups) > 1 {
			if _, err := buildFixProofWithDocument(text, doc, safeEdits, targets.Profiles(), validEdits); err != nil {
				return "", 0, fmt.Errorf("verify combined SafeFix edits: %w", err)
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

// proofContainsEdit is the linear membership scan the indexed authorization
// path is tested against.
func proofContainsEdit(proof *FixProof, candidate FixEditProof) bool {
	proofMembershipComparisons.Add(1)
	for _, sourceEdit := range proof.SourceEdits {
		proofMembershipComparisons.Add(1)
		if sourceEdit == candidate {
			return true
		}
	}
	return false
}
