package semantic

import (
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

type ProofValue struct {
	Value string `json:"value"`
	Known bool   `json:"known"`
}

type StateSnapshot struct {
	ActiveStyle string                `json:"active_style"`
	Values      map[string]ProofValue `json:"values"`
	Sources     map[string]int        `json:"sources,omitempty"`
}

// Snapshot copies the tracked values and their proven source indexes.
func (v StateView) Snapshot() StateSnapshot {
	values := make(map[string]ProofValue, 34)
	v.store.eachPresent(func(slot string) {
		values[slot] = proofValue(v.Value(slot))
	})
	for slot := range v.styles[v.activeStyle].Slots {
		values[slot] = proofValue(v.Value(slot))
	}
	for slot := range v.styles[v.activeStyle].Values {
		values[slot] = proofValue(v.Value(slot))
	}
	for _, slot := range []string{"drawing_scale", "karaoke_cursor"} {
		values[slot] = proofValue(v.Value(slot))
	}
	sources := make(map[string]int)
	v.store.eachOwner(func(slot string, witness owner) {
		if witness.proven {
			sources[slot] = witness.index
		}
	})
	return StateSnapshot{ActiveStyle: v.activeStyle, Values: values, Sources: sources}
}

func proofValue(value StateValue) ProofValue {
	return ProofValue(value)
}

type ProofStep struct {
	Kind          string                `json:"kind"`
	Source        ass.ConcreteSpan      `json:"source"`
	Index         int                   `json:"index,omitempty"`
	Text          string                `json:"text,omitempty"`
	Tag           string                `json:"tag,omitempty"`
	Raw           string                `json:"raw,omitempty"`
	Arguments     []string              `json:"arguments,omitempty"`
	Parenthesized bool                  `json:"parenthesized,omitempty"`
	InTransition  bool                  `json:"in_transition,omitempty"`
	ExtraSlashes  int                   `json:"extra_slashes,omitempty"`
	Match         string                `json:"match,omitempty"`
	Signature     string                `json:"signature,omitempty"`
	Citation      string                `json:"citation,omitempty"`
	Policy        string                `json:"policy,omitempty"`
	Slots         []string              `json:"slots,omitempty"`
	Before        map[string]ProofValue `json:"before,omitempty"`
	After         map[string]ProofValue `json:"after,omitempty"`
	Applied       bool                  `json:"applied,omitempty"`
	Known         bool                  `json:"known,omitempty"`
	Ignored       bool                  `json:"ignored,omitempty"`
	Barrier       bool                  `json:"barrier,omitempty"`
	Uncertainty   string                `json:"uncertainty,omitempty"`
	State         StateSnapshot         `json:"state"`
}

type ProofTrace struct {
	Steps []ProofStep `json:"steps"`
}

type ProofComparison struct {
	Equivalent bool       `json:"equivalent"`
	Reason     string     `json:"reason,omitempty"`
	Before     ProofTrace `json:"before"`
	After      ProofTrace `json:"after"`
}

// CompareResolvedProof compares one edit result within one pinned renderer.
// removed contains source ranges that the proposed edit removes from before.
// The decision runs on compact traces; the audit wire materializes only for
// an accepted comparison, byte-identical to the map pipeline it replaced.
func CompareResolvedProof(before, after ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions, removed []ass.ConcreteSpan) ProofComparison {
	beforeTrace := observeCompactDialogue(before, profile, options)
	afterTrace := observeCompactDialogue(after, profile, options)
	comparison := ProofComparison{}
	equivalent, reason := decideCompact(beforeTrace, afterTrace, removed)
	if !equivalent {
		comparison.Reason = reason
		return comparison
	}
	comparison.Equivalent = true
	comparison.Before = beforeTrace.wire()
	comparison.After = afterTrace.wire()
	return comparison
}

// CompareSyntaxProof verifies an edit that removes an empty override block.
// It compares complete renderer traces and rejects unresolved operations.
func CompareSyntaxProof(before, after ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) ProofComparison {
	beforeTrace := observeCompactDialogue(before, profile, options)
	afterTrace := observeCompactDialogue(after, profile, options)
	comparison := ProofComparison{}
	equivalent, reason := decideCompact(beforeTrace, afterTrace, nil)
	if !equivalent {
		comparison.Reason = reason
		return comparison
	}
	comparison.Equivalent = true
	comparison.Before = beforeTrace.wire()
	comparison.After = afterTrace.wire()
	return comparison
}

func slotValues(slots []string, values [4]StateValue) map[string]ProofValue {
	if len(slots) == 0 {
		return nil
	}
	out := make(map[string]ProofValue, len(slots))
	for i, slot := range slots {
		out[slot] = proofValue(values[i])
	}
	return out
}

func matchName(status renderer.MatchStatus) string {
	switch status {
	case renderer.Matched:
		return "matched"
	case renderer.Conditional:
		return "conditional"
	case renderer.Disabled:
		return "disabled"
	case renderer.Ignored:
		return "ignored"
	default:
		return "unknown-name"
	}
}

func signatureName(status renderer.SignatureStatus) string {
	switch status {
	case renderer.SignatureVerified:
		return "verified"
	case renderer.SignatureInferred:
		return "inferred"
	case renderer.SignatureRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

func behaviorName(behavior spec.Behavior) string {
	switch behavior {
	case spec.FirstWins:
		return "first-wins"
	case spec.Accumulate:
		return "accumulate"
	case spec.Transition:
		return "transition"
	case spec.StyleReset:
		return "style-reset"
	default:
		return "assign"
	}
}

func uncertaintyName(uncertainty SemanticUncertainty) string {
	switch uncertainty {
	case UncertaintyMalformed:
		return "malformed"
	case UncertaintyUnsupported:
		return "unsupported"
	case UncertaintyUnresolved:
		return "unresolved"
	case UncertaintyRendererDependent:
		return "renderer-dependent"
	case UncertaintyTimeDependent:
		return "time-dependent"
	default:
		return "none"
	}
}

func normalizeSlashPrefix(raw string) string {
	count := len(raw) - len(strings.TrimLeft(raw, "\\"))
	if count < 2 {
		return raw
	}
	return "\\" + raw[count:]
}
