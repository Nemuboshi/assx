package semantic

import (
	"reflect"
	"slices"
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
	values := make(map[string]ProofValue, len(v.values)+len(v.styles[v.activeStyle].Slots)+2)
	for slot := range v.styles[v.activeStyle].Slots {
		values[slot] = proofValue(v.Value(slot))
	}
	for slot := range v.styles[v.activeStyle].Values {
		values[slot] = proofValue(v.Value(slot))
	}
	for slot := range v.values {
		values[slot] = proofValue(v.Value(slot))
	}
	for _, slot := range []string{"drawing_scale", "karaoke_cursor"} {
		values[slot] = proofValue(v.Value(slot))
	}
	sources := make(map[string]int, len(v.sources))
	for slot, source := range v.sources {
		if source.proven {
			sources[slot] = source.index
		}
	}
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
func CompareResolvedProof(before, after ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions, removed []ass.ConcreteSpan) ProofComparison {
	beforeTrace := observeProofTrace(before, profile, options)
	afterTrace := observeProofTrace(after, profile, options)
	comparison := ProofComparison{Before: beforeTrace, After: afterTrace}
	if reason := proofTraceUncertainty(beforeTrace); reason != "" {
		comparison.Reason = "original source: " + reason
		return comparison
	}
	if reason := proofTraceUncertainty(afterTrace); reason != "" {
		comparison.Reason = "edited source: " + reason
		return comparison
	}
	beforeSteps := retainProofSteps(beforeTrace.Steps, removed)
	afterSteps := slicesCloneProofSteps(afterTrace.Steps)
	beforeSteps = normalizeProofSources(beforeSteps)
	afterSteps = normalizeProofSources(afterSteps)
	beforeSteps = mergeProofText(beforeSteps)
	afterSteps = mergeProofText(afterSteps)
	if !equalProofSteps(beforeSteps, afterSteps) {
		comparison.Reason = "renderer interpretation, state, provenance, or visible text changed"
		return comparison
	}
	comparison.Equivalent = true
	return comparison
}

// CompareSyntaxProof verifies an edit that removes an empty override block.
// It compares complete renderer traces and rejects unresolved operations.
func CompareSyntaxProof(before, after ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) ProofComparison {
	beforeTrace := observeProofTrace(before, profile, options)
	afterTrace := observeProofTrace(after, profile, options)
	comparison := ProofComparison{Before: beforeTrace, After: afterTrace}
	if reason := proofTraceUncertainty(beforeTrace); reason != "" {
		comparison.Reason = "original source: " + reason
		return comparison
	}
	if reason := proofTraceUncertainty(afterTrace); reason != "" {
		comparison.Reason = "edited source: " + reason
		return comparison
	}
	beforeSteps := normalizeProofSources(beforeTrace.Steps)
	afterSteps := normalizeProofSources(slicesCloneProofSteps(afterTrace.Steps))
	beforeSteps = mergeProofText(beforeSteps)
	afterSteps = mergeProofText(afterSteps)
	if !equalProofSteps(beforeSteps, afterSteps) {
		comparison.Reason = "renderer interpretation, state, provenance, or visible text changed"
		return comparison
	}
	comparison.Equivalent = true
	return comparison
}

func observeProofTrace(tree ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) ProofTrace {
	var trace ProofTrace
	options.Profile = nil
	options.Observer = Observer{
		Tag: func(event TagEvent, state StateView) {
			trace.Steps = append(trace.Steps, ProofStep{
				Kind: "tag", Source: ass.ConcreteSpan{Start: event.Tag.Start, End: event.Tag.End},
				Index: event.Index, Tag: event.Tag.Name, Raw: event.Tag.Raw,
				Arguments: slices.Clone(event.Tag.Args), Parenthesized: event.Tag.Paren,
				InTransition: event.Tag.InTransition, ExtraSlashes: event.Tag.RepeatedSlashes,
				Match: matchName(event.Match), Signature: signatureName(event.Signature),
				Citation: event.Citation, Policy: behaviorName(event.Policy),
				Slots: slices.Clone(event.Slots), Before: slotValues(event.Slots, event.Before),
				After:   slotValues(event.Slots, event.After),
				Applied: event.Applied, Known: event.Known, Ignored: event.Ignored,
				Barrier: event.Barrier, Uncertainty: uncertaintyName(event.Uncertainty), State: state.Snapshot(),
			})
		},
		Text: func(text string, start int, state StateView) {
			trace.Steps = append(trace.Steps, ProofStep{
				Kind: "text", Source: ass.ConcreteSpan{Start: start, End: start + len(text)},
				Text: text, State: state.Snapshot(),
			})
		},
	}
	EvaluateResolved(tree, profile, options)
	return trace
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

func proofTraceUncertainty(trace ProofTrace) string {
	for _, step := range trace.Steps {
		if step.Kind != "tag" {
			continue
		}
		if step.Barrier {
			return "a semantic barrier affects the proof"
		}
		knownRejectedForm := step.Uncertainty == "malformed" && step.Ignored && step.Signature == "rejected"
		if step.Uncertainty != "none" && !knownRejectedForm {
			return "a semantic uncertainty affects the proof"
		}
		if step.Match != "matched" {
			return "a command does not resolve as a verified match"
		}
		rejectedSignature := step.Ignored && step.Signature == "rejected"
		if step.Signature != "verified" && !rejectedSignature {
			return "a command signature is not verified"
		}
		if step.Citation == "" {
			return "a command has no pinned source citation"
		}
	}
	return ""
}

func retainProofSteps(steps []ProofStep, removed []ass.ConcreteSpan) []ProofStep {
	out := make([]ProofStep, 0, len(steps))
	for _, step := range steps {
		if step.Kind == "tag" && spanContainedByAny(step.Source, removed) {
			continue
		}
		out = append(out, step)
	}
	return out
}

func spanContainedByAny(span ass.ConcreteSpan, ranges []ass.ConcreteSpan) bool {
	for _, candidate := range ranges {
		if candidate.Start <= span.Start && span.End <= candidate.End {
			return true
		}
	}
	return false
}

func slicesCloneProofSteps(steps []ProofStep) []ProofStep {
	return slices.Clone(steps)
}

func normalizeProofSources(steps []ProofStep) []ProofStep {
	owners := make(map[int]int)
	ordinal := 0
	for _, step := range steps {
		if step.Kind == "tag" {
			owners[step.Index] = ordinal
			ordinal++
		}
	}
	for i := range steps {
		if len(steps[i].State.Sources) == 0 {
			continue
		}
		sources := make(map[string]int, len(steps[i].State.Sources))
		for slot, source := range steps[i].State.Sources {
			if normalized, ok := owners[source]; ok {
				sources[slot] = normalized
			} else {
				sources[slot] = -1
			}
		}
		steps[i].State.Sources = sources
	}
	return steps
}

func mergeProofText(steps []ProofStep) []ProofStep {
	out := make([]ProofStep, 0, len(steps))
	for _, step := range steps {
		if step.Kind == "text" && len(out) > 0 && out[len(out)-1].Kind == "text" &&
			reflect.DeepEqual(out[len(out)-1].State, step.State) {
			out[len(out)-1].Text += step.Text
			out[len(out)-1].Source.End = step.Source.End
			continue
		}
		out = append(out, step)
	}
	return out
}

func equalProofSteps(before, after []ProofStep) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		a, b := before[i], after[i]
		if a.Kind != b.Kind || a.Text != b.Text || !equalProofState(a.State, b.State) {
			return false
		}
		if a.Kind != "tag" {
			continue
		}
		if a.Tag != b.Tag || normalizeSlashPrefix(a.Raw) != normalizeSlashPrefix(b.Raw) ||
			!reflect.DeepEqual(a.Arguments, b.Arguments) || a.Parenthesized != b.Parenthesized ||
			a.InTransition != b.InTransition || a.Match != b.Match || a.Signature != b.Signature ||
			a.Citation != b.Citation || a.Policy != b.Policy || !reflect.DeepEqual(a.Slots, b.Slots) ||
			!reflect.DeepEqual(a.After, b.After) || a.Applied != b.Applied || a.Known != b.Known ||
			a.Ignored != b.Ignored || a.Barrier != b.Barrier || a.Uncertainty != b.Uncertainty {
			return false
		}
	}
	return true
}

func equalProofState(a, b StateSnapshot) bool {
	if a.ActiveStyle != b.ActiveStyle || !reflect.DeepEqual(a.Values, b.Values) {
		return false
	}
	for slot, source := range a.Sources {
		other, rightOK := b.Sources[slot]
		if !rightOK {
			if source == -1 {
				continue
			}
			return false
		}
		if source != other {
			return false
		}
	}
	for slot := range b.Sources {
		if _, ok := a.Sources[slot]; !ok {
			return false
		}
	}
	return true
}

func normalizeSlashPrefix(raw string) string {
	count := len(raw) - len(strings.TrimLeft(raw, "\\"))
	if count < 2 {
		return raw
	}
	return "\\" + raw[count:]
}
