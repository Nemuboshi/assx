// Package analysis coordinates one snapshot analysis: discovery through
// internal/lint, deferred SafeFix proof, and policy-aware application. It
// owns request state only. Decoding, encoding, font discovery, reports, and
// flags belong to their current owners.
package analysis

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"slices"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/lint"
)

// Options selects the analysis targets and rule policy. Empty values use
// the default targets and the default rule catalog.
type Options struct {
	Targets lint.AnalysisTargets
	Rules   lint.RulePolicy
}

// FindingID names one finding in one result. The identity is opaque; an ID
// from another result fails.
type FindingID struct {
	result  uint64
	ordinal int
}

// Finding is one diagnostic plus its candidate handle and flag. A pending
// diagnostic carries no unproved SafeFix label, proof, or executable edit.
type Finding struct {
	ID              FindingID
	Diagnostic      lint.Diagnostic
	HasFixCandidate bool

	candidate lint.CandidateID
}

// Result owns one analysis of one snapshot: findings, options, and the lint
// session that proves and applies candidates.
type Result struct {
	id       uint64
	snapshot Snapshot
	options  Options
	session  *lint.Session
	findings []Finding

	// candidateOrdinals lists finding ordinals that hold fix candidates.
	candidateOrdinals []int
}

// Analyze discovers findings for one snapshot under the given options. It
// never constructs a SafeFix proof. It rejects an invalid option set and
// honors cancellation.
func Analyze(ctx context.Context, snapshot Snapshot, options Options) (*Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := options.Rules.Validate(); err != nil {
		return nil, err
	}
	targets := options.Targets
	// The zero target shape (zero-value primary, nil compatibility) means the
	// current default targets. The libass profile is the zero value, so an
	// explicit selection must pass a non-nil compatibility slice, even an
	// empty one.
	if targets.Primary == (renderer.Profile{}) && targets.Compatibility == nil {
		targets = lint.DefaultAnalysisTargets()
	}
	targets.Compatibility = slices.Clone(targets.Compatibility)
	policyValue := slices.Clone(options.Rules)
	doc := ass.Parse(snapshot.Text())
	session, err := lint.Discover(ctx, doc, targets, policyValue)
	if err != nil {
		return nil, err
	}
	result := &Result{
		id:       newResultID(),
		snapshot: snapshot,
		options:  Options{Targets: targets, Rules: policyValue},
		session:  session,
	}
	pending := session.Findings()
	result.findings = make([]Finding, len(pending))
	for i, finding := range pending {
		result.findings[i] = Finding{
			ID:              FindingID{result: result.id, ordinal: i},
			Diagnostic:      finding.Diagnostic,
			HasFixCandidate: finding.HasFixCandidate,
			candidate:       finding.Candidate,
		}
		if finding.HasFixCandidate {
			result.candidateOrdinals = append(result.candidateOrdinals, i)
		}
	}
	return result, nil
}

// Findings returns copied findings in report order. Mutating a result never
// changes result state.
func (r *Result) Findings() []Finding {
	return slices.Clone(r.findings)
}

// ProveFixes proves the requested candidate findings as one batch and
// returns their authorized diagnostics in report order. A candidate that
// cannot be proved returns its diagnostic without automatic edits.
func (r *Result) ProveFixes(ctx context.Context, ids []FindingID) ([]lint.Diagnostic, error) {
	handles, err := r.handles(ids)
	if err != nil {
		return nil, err
	}
	return r.session.ProveFixes(ctx, handles)
}

// Diagnostics proves every candidate and merges the result into the pending
// findings. The output equals the eager analysis of the same snapshot,
// targets, and policy.
func (r *Result) Diagnostics(ctx context.Context) ([]lint.Diagnostic, error) {
	handles, err := r.handles(nil)
	if err != nil {
		return nil, err
	}
	authorized, err := r.session.ProveFixes(ctx, handles)
	if err != nil {
		return nil, err
	}
	out := slices.Clone(r.findings)
	for k, diagnostic := range authorized {
		out[r.candidateOrdinals[k]].Diagnostic = diagnostic
	}
	diagnostics := make([]lint.Diagnostic, len(out))
	for i, finding := range out {
		diagnostics[i] = finding.Diagnostic
	}
	return diagnostics, nil
}

// handles maps finding IDs to session candidate handles. A nil ids list
// selects every candidate, in report order.
func (r *Result) handles(ids []FindingID) ([]lint.CandidateID, error) {
	if ids == nil {
		handles := make([]lint.CandidateID, len(r.candidateOrdinals))
		for i, ordinal := range r.candidateOrdinals {
			handles[i] = r.findings[ordinal].candidate
		}
		return handles, nil
	}
	handles := make([]lint.CandidateID, 0, len(ids))
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if id.result != r.id || id.ordinal < 0 || id.ordinal >= len(r.findings) {
			return nil, fmt.Errorf("unknown finding id")
		}
		if seen[id.ordinal] {
			return nil, fmt.Errorf("duplicate finding id")
		}
		seen[id.ordinal] = true
		if !r.findings[id.ordinal].HasFixCandidate {
			return nil, fmt.Errorf("finding at ordinal %d is not a fix candidate", id.ordinal)
		}
		handles = append(handles, r.findings[id.ordinal].candidate)
	}
	return handles, nil
}

// ApplyFixes writes authorized edits into the snapshot source. It performs
// no file access. Every SafeFix finding must match a session candidate, and
// every proof re-verifies against the result targets and rule policy.
func (r *Result) ApplyFixes(ctx context.Context, diagnostics []lint.Diagnostic, includeUnsafe bool) (string, int, error) {
	return r.session.ApplyFixes(ctx, diagnostics, includeUnsafe)
}

func newResultID() uint64 {
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
