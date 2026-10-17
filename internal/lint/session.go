package lint

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"slices"

	"assx/internal/ass"
)

// CandidateID names one fix candidate in one session. The identity is opaque
// outside this package; a handle from another session fails.
type CandidateID struct {
	session uint64
	ordinal int
}

// PendingFinding is one diagnostic plus its candidate handle. A pending
// diagnostic carries no unproved SafeFix label, proof, or executable edit.
type PendingFinding struct {
	Diagnostic      Diagnostic
	Candidate       CandidateID
	HasFixCandidate bool
}

// Session holds one discovery pass: pending findings, fix candidates with
// their edits, analysis targets, and rule policy. Proof and application use
// the stored document source.
type Session struct {
	id          uint64
	doc         ass.Document
	targets     AnalysisTargets
	policy      RulePolicy
	findings    []Diagnostic
	candidates  []Diagnostic
	candidateOf []int
}

func newSessionID() uint64 {
	for {
		var buffer [8]byte
		if _, err := rand.Read(buffer[:]); err != nil {
			panic(err)
		}
		if id := binary.LittleEndian.Uint64(buffer[:]); id != 0 {
			return id
		}
	}
}

// Discover runs every lint pass and records fix candidates without proving
// them. It rejects an invalid policy and honors cancellation.
func Discover(ctx context.Context, doc ass.Document, targets AnalysisTargets, policy RulePolicy) (*Session, error) {
	if err := policy.validate(); err != nil {
		return nil, err
	}
	findings, err := analyzeDocumentForTargetsUnprovedContext(ctx, doc, targets, policy)
	if err != nil {
		return nil, err
	}
	session := &Session{id: newSessionID(), doc: doc, targets: targets, policy: policy}
	for index, diagnostic := range findings {
		if diagnostic.FixSafety != SafeFix || len(diagnostic.Edits) == 0 {
			// Proof strips a SafeFix label without edits, as today.
			pending := diagnostic
			if pending.FixSafety == SafeFix {
				pending.FixSafety = ""
				pending.FixProof = nil
			}
			session.findings = append(session.findings, pending)
			continue
		}
		session.candidates = append(session.candidates, cloneDiagnostic(diagnostic))
		session.candidateOf = append(session.candidateOf, index)
		pending := diagnostic
		clearPendingFix(&pending)
		session.findings = append(session.findings, pending)
	}
	return session, nil
}

func clearPendingFix(diagnostic *Diagnostic) {
	diagnostic.FixSafety = ""
	diagnostic.FixProof = nil
	diagnostic.FixProofRef = ""
	diagnostic.Edits = nil
}

// Findings returns copied pending findings in report order. Mutating a
// result never changes session state.
func (s *Session) Findings() []PendingFinding {
	out := make([]PendingFinding, len(s.findings))
	candidate := 0
	for i := range s.findings {
		finding := PendingFinding{Diagnostic: cloneDiagnostic(s.findings[i])}
		if candidate < len(s.candidateOf) && s.candidateOf[candidate] == i {
			finding.Candidate = CandidateID{session: s.id, ordinal: candidate}
			finding.HasFixCandidate = true
			candidate++
		}
		out[i] = finding
	}
	return out
}

// ProveFixes builds proofs for the requested candidates as one batch, in
// report order. It clones stored candidates, so repeated calls stay
// independent and never accept caller-supplied edits as authority.
func (s *Session) ProveFixes(ctx context.Context, handles []CandidateID) ([]Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ordinals, err := s.resolve(handles)
	if err != nil {
		return nil, err
	}
	batch := make([]Diagnostic, len(ordinals))
	for i, ordinal := range ordinals {
		batch[i] = cloneDiagnostic(s.candidates[ordinal])
	}
	return proveSafeFixesContext(ctx, s.doc, batch, s.targets.Profiles())
}

func (s *Session) resolve(handles []CandidateID) ([]int, error) {
	seen := make(map[int]bool, len(handles))
	ordinals := make([]int, 0, len(handles))
	for _, handle := range handles {
		if handle.session != s.id || handle.ordinal < 0 || handle.ordinal >= len(s.candidates) {
			return nil, fmt.Errorf("unknown fix candidate")
		}
		if seen[handle.ordinal] {
			return nil, fmt.Errorf("duplicate fix candidate")
		}
		seen[handle.ordinal] = true
		ordinals = append(ordinals, handle.ordinal)
	}
	slices.Sort(ordinals)
	return ordinals, nil
}

// ApplyFixes writes authorized edits into the session source. It performs no
// file access. Every SafeFix finding must match a stored session candidate,
// and every proof re-verifies against the session targets and rule policy.
func (s *Session) ApplyFixes(ctx context.Context, diagnostics []Diagnostic, includeUnsafe bool) (string, int, error) {
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if err := s.authorize(diagnostics); err != nil {
		return "", 0, err
	}
	return applyFixes(s.doc.Text, diagnostics, includeUnsafe, s.targets, s.policy)
}

type candidateKey struct {
	id    string
	edits string
}

func candidateKeyOf(diagnostic Diagnostic) candidateKey {
	return candidateKey{id: diagnostic.ID, edits: fmt.Sprintf("%v", diagnostic.Edits)}
}

func (s *Session) authorize(diagnostics []Diagnostic) error {
	pool := make(map[candidateKey]int, len(s.candidates))
	for _, candidate := range s.candidates {
		pool[candidateKeyOf(candidate)]++
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety != SafeFix || len(diagnostic.Edits) == 0 {
			continue
		}
		key := candidateKeyOf(diagnostic)
		if pool[key] == 0 {
			return fmt.Errorf("SafeFix finding %s is not a session candidate", diagnostic.ID)
		}
		pool[key]--
	}
	return nil
}

func cloneDiagnostic(diagnostic Diagnostic) Diagnostic {
	out := diagnostic
	out.Sources = slices.Clone(diagnostic.Sources)
	out.Edits = slices.Clone(diagnostic.Edits)
	if diagnostic.SourceSpan != nil {
		span := *diagnostic.SourceSpan
		out.SourceSpan = &span
	}
	return out
}
