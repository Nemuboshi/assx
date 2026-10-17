package semantic

import (
	"slices"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// Compact proof traces carry one evaluation as indexed slices instead of
// wire maps. The authorization decision compares compact traces; the audit
// wire (json.Marshal output and identity hashes) materializes full ProofStep
// values only at the accepted-edit boundary, byte-identical to the map
// pipeline that produced them. Rejected comparisons materialize nothing.

type compactKind uint8

const (
	compactTagStep compactKind = iota
	compactTextStep
)

type compactEntry struct {
	slot  string
	value StateValue
}

type compactOwner struct {
	slot  string
	index int
}

type compactState struct {
	style  string
	values []compactEntry
	owners []compactOwner
}

type compactTag struct {
	name, raw     string
	args          []string
	paren         bool
	inTransition  bool
	extraSlashes  int
	match         string
	signature     string
	citation      string
	policy        string
	slots         []string
	before, after [4]StateValue
	applied       bool
	known         bool
	ignored       bool
	barrier       bool
	uncertainty   SemanticUncertainty
}

type compactStep struct {
	kind    compactKind
	source  ass.ConcreteSpan
	ordinal int
	text    string
	tag     compactTag
	state   compactState
}

type compactTrace struct {
	steps []compactStep
}

// observeCompactDialogue records one evaluation without building any map.
func observeCompactDialogue(tree ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) compactTrace {
	trace := compactTrace{}
	options.Observer = Observer{
		Tag: func(event TagEvent, state StateView) {
			trace.steps = append(trace.steps, compactStep{
				kind:    compactTagStep,
				source:  ass.ConcreteSpan{Start: event.Tag.Start, End: event.Tag.End},
				ordinal: event.Index,
				tag: compactTag{
					name: event.Tag.Name, raw: event.Tag.Raw, args: slices.Clone(event.Tag.Args),
					paren: event.Tag.Paren, inTransition: event.Tag.InTransition,
					extraSlashes: event.Tag.RepeatedSlashes,
					match:        matchName(event.Match), signature: signatureName(event.Signature),
					citation: event.Citation, policy: behaviorName(event.Policy),
					slots: slices.Clone(event.Slots), before: event.Before, after: event.After,
					applied: event.Applied, known: event.Known, ignored: event.Ignored,
					barrier: event.Barrier, uncertainty: event.Uncertainty,
				},
				state: captureCompactState(state),
			})
		},
		Text: func(text string, start int, state StateView) {
			trace.steps = append(trace.steps, compactStep{
				kind: compactTextStep, source: ass.ConcreteSpan{Start: start, End: start + len(text)},
				text: text, state: captureCompactState(state),
			})
		},
	}
	EvaluateResolved(tree, profile, options)
	return trace
}

// captureCompactState freezes the mutable store into canonically ordered
// entries. The wire Snapshot expands the same rule set: present store
// values, the active Style's declared and decoded slots, then the two
// built-in defaults.
func captureCompactState(v StateView) compactState {
	state := compactState{style: v.activeStyle}
	var flags [maxSlots]bool
	var fallback []string
	add := func(slot string) {
		if id, ok := slotIndex(slot); ok {
			flags[id] = true
			return
		}
		if !slices.Contains(fallback, slot) {
			fallback = append(fallback, slot)
		}
	}
	v.store.eachPresent(add)
	for slot := range v.styles[v.activeStyle].Slots {
		add(slot)
	}
	for slot := range v.styles[v.activeStyle].Values {
		add(slot)
	}
	add("drawing_scale")
	add("karaoke_cursor")
	for id := slotID(0); id < maxSlots; id++ {
		if flags[id] {
			slot := slotName(id)
			state.values = append(state.values, compactEntry{slot: slot, value: v.Value(slot)})
		}
	}
	slices.Sort(fallback)
	for _, slot := range fallback {
		state.values = append(state.values, compactEntry{slot: slot, value: v.Value(slot)})
	}
	v.store.eachOwner(func(slot string, witness owner) {
		if witness.proven {
			state.owners = append(state.owners, compactOwner{slot: slot, index: witness.index})
		}
	})
	slices.SortFunc(state.owners, func(a, b compactOwner) int { return strings.Compare(a.slot, b.slot) })
	return state
}

// stateEqualProof mirrors equalProofState: after normalization a missing
// provenance key and an explicit -1 witness are interchangeable, on the left
// only; a provenance present on the right must exist on the left.
func stateEqualProof(a, b compactState) bool {
	if a.style != b.style || !entriesEqual(a.values, b.values) {
		return false
	}
	for _, entry := range a.owners {
		other, found := lookupOwner(b.owners, entry.slot)
		if !found {
			if entry.index == -1 {
				continue
			}
			return false
		}
		if entry.index != other.index {
			return false
		}
	}
	for _, other := range b.owners {
		if _, found := lookupOwner(a.owners, other.slot); !found {
			return false
		}
	}
	return true
}

func lookupOwner(entries []compactOwner, slot string) (compactOwner, bool) {
	i := slices.IndexFunc(entries, func(e compactOwner) bool { return e.slot == slot })
	if i < 0 {
		return compactOwner{}, false
	}
	return entries[i], true
}

// stateEqualStrict mirrors reflect.DeepEqual on the wire StateSnapshot, the
// merge rule for adjacent text steps.
func stateEqualStrict(a, b compactState) bool {
	return a.style == b.style && entriesEqual(a.values, b.values) && ownersEqual(a.owners, b.owners)
}

func entriesEqual(a, b []compactEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func ownersEqual(a, b []compactOwner) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalCompactSteps(before, after []compactStep) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		a, b := before[i], after[i]
		if a.kind != b.kind || a.text != b.text || !stateEqualProof(a.state, b.state) {
			return false
		}
		if a.kind != compactTagStep {
			continue
		}
		x, y := a.tag, b.tag
		if x.name != y.name || normalizeSlashPrefix(x.raw) != normalizeSlashPrefix(y.raw) ||
			!slices.Equal(x.args, y.args) || x.paren != y.paren ||
			x.inTransition != y.inTransition || x.match != y.match || x.signature != y.signature ||
			x.citation != y.citation || x.policy != y.policy || !slices.Equal(x.slots, y.slots) ||
			x.after != y.after || x.applied != y.applied || x.known != y.known ||
			x.ignored != y.ignored || x.barrier != y.barrier || x.uncertainty != y.uncertainty {
			return false
		}
	}
	return true
}

func materializeCompactState(state compactState) StateSnapshot {
	values := make(map[string]ProofValue, len(state.values))
	for _, entry := range state.values {
		values[entry.slot] = proofValue(entry.value)
	}
	sources := make(map[string]int, len(state.owners))
	for _, entry := range state.owners {
		sources[entry.slot] = entry.index
	}
	return StateSnapshot{ActiveStyle: state.style, Values: values, Sources: sources}
}

func materializeCompactStep(step compactStep) ProofStep {
	out := ProofStep{State: materializeCompactState(step.state)}
	if step.kind == compactTextStep {
		out.Kind = "text"
		out.Source = step.source
		out.Text = step.text
		return out
	}
	tag := step.tag
	out.Kind = "tag"
	out.Source = step.source
	out.Index = step.ordinal
	out.Tag = tag.name
	out.Raw = tag.raw
	out.Arguments = tag.args
	out.Parenthesized = tag.paren
	out.InTransition = tag.inTransition
	out.ExtraSlashes = tag.extraSlashes
	out.Match = tag.match
	out.Signature = tag.signature
	out.Citation = tag.citation
	out.Policy = tag.policy
	out.Slots = tag.slots
	out.Before = slotValues(tag.slots, tag.before)
	out.After = slotValues(tag.slots, tag.after)
	out.Applied = tag.applied
	out.Known = tag.known
	out.Ignored = tag.ignored
	out.Barrier = tag.barrier
	out.Uncertainty = uncertaintyName(tag.uncertainty)
	return out
}

func (t compactTrace) wire() ProofTrace {
	out := make([]ProofStep, 0, len(t.steps))
	for _, step := range t.steps {
		out = append(out, materializeCompactStep(step))
	}
	return ProofTrace{Steps: out}
}

func compactUncertainty(step compactStep) string {
	if step.kind != compactTagStep {
		return ""
	}
	tag := step.tag
	if tag.barrier {
		return "a semantic barrier affects the proof"
	}
	knownRejectedForm := tag.uncertainty == UncertaintyMalformed && tag.ignored && tag.signature == "rejected"
	if tag.uncertainty != UncertaintyNone && !knownRejectedForm {
		return "a semantic uncertainty affects the proof"
	}
	if tag.match != "matched" {
		return "a command does not resolve as a verified match"
	}
	rejectedSignature := tag.ignored && tag.signature == "rejected"
	if tag.signature != "verified" && !rejectedSignature {
		return "a command signature is not verified"
	}
	if tag.citation == "" {
		return "a command has no pinned source citation"
	}
	return ""
}

func compactTraceUncertainty(trace compactTrace) string {
	for _, step := range trace.steps {
		if reason := compactUncertainty(step); reason != "" {
			return reason
		}
	}
	return ""
}

func retainCompact(steps []compactStep, removed []ass.ConcreteSpan) []compactStep {
	out := make([]compactStep, 0, len(steps))
	for _, step := range steps {
		if step.kind == compactTagStep && spanContainedByAny(step.source, removed) {
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

func normalizeCompact(steps []compactStep) []compactStep {
	owners := make(map[int]int, len(steps))
	ordinal := 0
	for _, step := range steps {
		if step.kind == compactTagStep {
			owners[step.ordinal] = ordinal
			ordinal++
		}
	}
	for i := range steps {
		if len(steps[i].state.owners) == 0 {
			continue
		}
		normalized := make([]compactOwner, 0, len(steps[i].state.owners))
		for _, entry := range steps[i].state.owners {
			mapped, ok := owners[entry.index]
			if !ok {
				mapped = -1
			}
			normalized = append(normalized, compactOwner{slot: entry.slot, index: mapped})
		}
		steps[i].state.owners = normalized
	}
	return steps
}

func mergeCompact(steps []compactStep) []compactStep {
	out := make([]compactStep, 0, len(steps))
	for _, step := range steps {
		if step.kind == compactTextStep && len(out) > 0 && out[len(out)-1].kind == compactTextStep &&
			stateEqualStrict(out[len(out)-1].state, step.state) {
			out[len(out)-1].text += step.text
			out[len(out)-1].source.End = step.source.End
			continue
		}
		out = append(out, step)
	}
	return out
}

// decideCompact runs the authorization pipeline on compact traces: gate,
// retain, normalize, merge, then field-for-field comparison. Wire traces
// materialize only when a caller asks (an accepted comparison).
func decideCompact(before, after compactTrace, removed []ass.ConcreteSpan) (bool, string) {
	if reason := compactTraceUncertainty(before); reason != "" {
		return false, "original source: " + reason
	}
	if reason := compactTraceUncertainty(after); reason != "" {
		return false, "edited source: " + reason
	}
	beforeSteps := normalizeCompact(retainCompact(before.steps, removed))
	afterSteps := normalizeCompact(slices.Clone(after.steps))
	beforeSteps = mergeCompact(beforeSteps)
	afterSteps = mergeCompact(afterSteps)
	if !equalCompactSteps(beforeSteps, afterSteps) {
		return false, "renderer interpretation, state, provenance, or visible text changed"
	}
	return true, ""
}
