package semantic

// Test-only reference pipeline. The production comparison runs on compact
// traces (compact_proof.go); these full-map functions are the differential
// oracle that kept the compact decision byte-compatible during migration.

import (
	"reflect"
	"slices"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func observeProofTrace(tree ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) ProofTrace {
	var trace ProofTrace
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

func referenceCompareResolvedProof(before, after ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions, removed []ass.ConcreteSpan) ProofComparison {
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
	afterSteps := slices.Clone(afterTrace.Steps)
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

func referenceCompareSyntaxProof(before, after ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) ProofComparison {
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
	afterSteps := normalizeProofSources(slices.Clone(afterTrace.Steps))
	beforeSteps = mergeProofText(beforeSteps)
	afterSteps = mergeProofText(afterSteps)
	if !equalProofSteps(beforeSteps, afterSteps) {
		comparison.Reason = "renderer interpretation, state, provenance, or visible text changed"
		return comparison
	}
	comparison.Equivalent = true
	return comparison
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
