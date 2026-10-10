package semantic

import (
	"slices"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

type NoEffectReason uint8

const (
	SameValue NoEffectReason = iota
	FirstWinsIgnored
	OverwrittenBeforeUse
	ResetBeforeUse
	TransitionNoEffect
)

type NoEffect struct {
	Tag          ass.Tag
	Reason       NoEffectReason
	OwnerIndex   int
	ProofRevoked bool // An unmodeled operation invalidated the SafeFix proof.
}

// SemanticUncertainty records why a tag cannot be modeled exactly.
// A nonzero value does not by itself imply a dialogue-wide SafeFix barrier:
// time-dependent transforms are represented separately from proof failures.
type SemanticUncertainty uint8

const (
	UncertaintyNone SemanticUncertainty = iota
	UncertaintyMalformed
	UncertaintyUnsupported
	UncertaintyUnresolved
	UncertaintyRendererDependent
	UncertaintyTimeDependent
)

// TagEvent describes the state transition performed by one source tag.
// Before and After correspond to Slots, in the same order. Observers must not
// retain the slices or mutate the state view.
type TagEvent struct {
	Tag         ass.Tag
	Index       int
	Policy      spec.Behavior
	Slots       []string
	Before      [4]StateValue
	After       [4]StateValue
	Applied     bool
	Known       bool
	Barrier     bool
	Uncertainty SemanticUncertainty
	// Resolution metadata is separate from value certainty and proof status.
	Renderer  renderer.Kind
	Match     renderer.MatchStatus
	Signature renderer.SignatureStatus
	Shadowed  string
	Ignored   bool
}

// StateView is valid only during an observer callback. It exposes the same
// authoritative state and source provenance to lint and future formatters.
type StateView struct {
	values        map[string]StateValue
	sources       map[string]owner
	styles        map[string]StyleState
	activeStyle   string
	originalStyle string
	baseValid     bool
}

func (v StateView) Value(slot string) StateValue {
	if value, exists := v.values[slot]; exists {
		return value
	}
	if !v.baseValid {
		return UnknownValue()
	}
	switch slot {
	case "drawing_scale", "karaoke_cursor":
		return KnownValue("0")
	default:
		return v.styles[v.activeStyle].Values[slot]
	}
}
func (v StateView) Source(slot string) int {
	if source, ok := v.sources[slot]; ok && source.proven {
		return source.index
	}
	return -1
}
func (v StateView) ActiveStyle() string   { return v.activeStyle }
func (v StateView) OriginalStyle() string { return v.originalStyle }
func (v StateView) Style() StyleState     { return v.styles[v.activeStyle] }
func (v StateView) Original() StyleState  { return v.styles[v.originalStyle] }

// Observer receives each tag after its transition and each source text
// boundary after the preceding tags. No snapshots are allocated for text
// unless the consumer explicitly needs to retain a value.
type Observer struct {
	Tag  func(TagEvent, StateView)
	Text func(text string, start int, state StateView)
}

type EvaluationOptions struct {
	Styles        map[string]StyleState
	DialogueStyle string
	Observer      Observer
	// SkipNoEffectProofs avoids provenance/liveness bookkeeping when only
	// effective text state is needed (for example, font coverage analysis).
	SkipNoEffectProofs bool
	// Profile selects independent renderer resolution. Nil preserves the
	// historical default lint contract until profile-scoped validation in P06.
	Profile *renderer.Profile
}

type Evaluation struct {
	NoEffects []NoEffect
}

type owner struct {
	index  int
	start  int
	proven bool
}

type noEffectCandidate struct {
	tag        ass.Tag
	reason     NoEffectReason
	ownerIndex int
	always     bool
}

// evaluator is the sole owner of style resolution, precedence, effective
// values, and source-tag provenance. Lint consumers never replay assignments.
type evaluator struct {
	options           EvaluationOptions
	position          int
	collisionDisabled bool
	proofsDisabled    bool
	tagIndex          int
	active            map[string]owner
	state             map[string]StateValue
	latched           map[string]int
	uncertainLatches  map[string]bool
	allTags           []ass.Tag
	live              map[int]bool
	candidates        map[int]noEffectCandidate
	originalStyle     string
	activeStyle       string
	baseValid         bool
	operations        map[int]resolvedOperation
}

func EvaluateDialogue(tree ass.DialogueText) []NoEffect {
	return Evaluate(tree, EvaluationOptions{}).NoEffects
}

func Evaluate(tree ass.DialogueText, options EvaluationOptions) Evaluation {
	return evaluate(tree, nil, options)
}

// EvaluateResolved evaluates an existing lossless CST for one pinned renderer.
// The CST is never named or rewritten to match a profile. All observers and
// NoEffect results come from the same state engine as the default entry point.
func EvaluateResolved(tree ass.ConcreteDialogue, profile renderer.Profile, options EvaluationOptions) Evaluation {
	options.Profile = &profile
	return evaluate(ass.DialogueText{Source: tree.Source}, &tree, options)
}

func evaluate(tree ass.DialogueText, concrete *ass.ConcreteDialogue, options EvaluationOptions) Evaluation {
	if options.Observer.Text == nil {
		if options.Profile == nil && !tree.HasTags() {
			return Evaluation{}
		}
		if options.Profile != nil {
			if concrete != nil && !concrete.HasCandidates() {
				return Evaluation{}
			}
			if concrete == nil && (!strings.Contains(tree.Source, "{") || !strings.Contains(tree.Source, "\\")) {
				return Evaluation{}
			}
		}
	}
	name := DialogueStyleLookupName(options.DialogueStyle)
	m := evaluator{
		options:          options,
		active:           make(map[string]owner),
		state:            make(map[string]StateValue),
		latched:          make(map[string]int),
		uncertainLatches: make(map[string]bool),
		live:             make(map[int]bool),
		candidates:       make(map[int]noEffectCandidate),
		originalStyle:    name,
		activeStyle:      name,
		baseValid:        true,
	}
	if options.SkipNoEffectProofs {
		m.proofsDisabled = true
	}
	// Style values are read through the active base on demand. Copying every
	// Style slot into each Dialogue would dominate allocations for large files.
	visit := func(token semanticToken) {
		if token.hasTag {
			event := m.consumeTag(token.tag)
			if options.Observer.Tag != nil {
				options.Observer.Tag(event, m.view())
			}
		} else {
			m.consumeText(token.text)
			if options.Observer.Text != nil {
				options.Observer.Text(token.text, token.start, m.view())
			}
		}
	}
	if options.Profile == nil {
		tree.WalkTokens(func(token ass.TokenView) bool {
			visit(semanticToken{text: token.Text, start: token.Start, tag: token.Tag, hasTag: token.HasTag})
			return true
		})
	} else {
		if concrete == nil {
			parsed := ass.ParseConcreteDialogue(tree.Source)
			concrete = &parsed
		}
		tokens, operations := resolveTokens(*concrete, *options.Profile)
		m.operations = operations
		for _, token := range tokens {
			visit(token)
		}
	}
	out := make([]NoEffect, 0, len(m.candidates))
	for index := range m.allTags {
		candidate, ok := m.candidates[index]
		if !ok || (!candidate.always && m.live[index]) {
			continue
		}
		out = append(out, NoEffect{
			Tag: candidate.tag, Reason: candidate.reason, OwnerIndex: candidate.ownerIndex,
			ProofRevoked: m.proofsDisabled,
		})
	}
	return Evaluation{NoEffects: out}
}

func (m *evaluator) view() StateView {
	return StateView{
		values: m.state, sources: m.active, styles: m.options.Styles,
		activeStyle: m.activeStyle, originalStyle: m.originalStyle,
		baseValid: m.baseValid,
	}
}

func (m *evaluator) consumeText(text string) {
	if text != "" {
		m.markActiveLive()
		m.position++
	}
}

func (m *evaluator) markActiveLive() {
	if m.options.SkipNoEffectProofs {
		return
	}
	for _, active := range m.active {
		if active.index >= 0 {
			m.live[active.index] = true
		}
	}
}

func (m *evaluator) consumeTag(tag ass.Tag) TagEvent {
	index := m.tagIndex
	m.tagIndex++
	if !m.options.SkipNoEffectProofs {
		m.allTags = append(m.allTags, tag)
	}
	event := TagEvent{Tag: tag, Index: index}

	if tag.RepeatedSlashes > 0 {
		if m.options.Profile != nil {
			// A slash run does not establish a normally resolved operation.
			// Do not silently treat it as a harmless ignored command.
			m.markActiveLive()
			m.invalidateAll()
			m.blockProofs(&event, UncertaintyUnresolved)
		}
		return event // The legacy parser separately emits the normalized tag.
	}

	tagSpec, known := m.policyFor(tag)
	op := m.operationFor(tag)
	if m.options.Profile != nil {
		event.Renderer = m.options.Profile.Kind()
		event.Match, event.Signature, event.Shadowed = op.match, op.signature, op.shadowed
		// Resolution cannot name an unclosed parenthesized expression. It is
		// malformed syntax, not a well-formed, harmless unknown command.
		// Handle this before the UnknownName/Ignored fast path so malformed
		// operations revoke earlier as well as subsequent SafeFix proofs.
		if op.unclosed {
			m.markActiveLive()
			if tag.Name == "clip" || tag.Name == "iclip" {
				// Either rectangular or vector clipping may have been affected.
				// Keep unrelated Style/font state intact, but revoke both clip
				// values, source owners and certainty of vector first-wins.
				m.invalidateMalformedTag(spec.TagSpec{Semantic: spec.SemanticClip}, index)
			} else {
				// No trustworthy resolved policy exists for this incomplete
				// command (notably an unclosed transform).
				m.invalidateAll()
			}
			m.blockProofs(&event, UncertaintyMalformed)
			return event
		}
		switch op.match {
		case renderer.UnknownName, renderer.Ignored, renderer.Disabled:
			event.Ignored = true
			event.Uncertainty = UncertaintyUnsupported
			return event
		case renderer.Conditional:
			m.markActiveLive()
			m.invalidateAll()
			m.blockProofs(&event, UncertaintyRendererDependent)
			return event
		}
		if op.signature == renderer.SignatureRejected {
			// Pinned exhaustive evidence proves this form cannot claim any
			// state or first-wins latch; subsequent valid operations still run.
			event.Ignored = true
			event.Uncertainty = UncertaintyMalformed
			return event
		}
		if !known || tagSpec.VSFilterModOnly && len(tagSpec.Slots) == 0 {
			m.markActiveLive()
			m.invalidateAll()
			m.blockProofs(&event, UncertaintyUnsupported)
			return event
		}
	} else if !known || tag.Name == "N" || tag.Name == "n" || tag.Name == "h" || tagSpec.VSFilterModOnly {
		// Unknown renderer prefixes can change arbitrary state. Do not prove
		// any later fix on the strength of the previous state.
		m.markActiveLive()
		m.invalidateAll()
		reason := UncertaintyUnsupported
		if tagSpec.VSFilterModOnly {
			reason = UncertaintyRendererDependent
		}
		m.blockProofs(&event, reason)
		return event
	}
	if tagSpec.Counts != nil && !slices.Contains(tagSpec.Counts, len(tag.Args)) &&
		(m.options.Profile == nil || op.signature != renderer.SignatureVerified) {
		if m.options.Profile != nil && tagSpec.Behavior == spec.FirstWins {
			// A definite owner wins regardless of whether a later form is
			// supported in this build. Preserve both value and provenance.
			owned := true
			for _, slot := range tagSpec.Slots {
				if _, exists := m.latched[slot]; !exists {
					owned = false
					break
				}
			}
			if owned && len(tagSpec.Slots) != 0 {
				event.Ignored = true
				event.Uncertainty = UncertaintyUnresolved
				return event
			}
		}
		m.invalidateMalformedTag(tagSpec, index)
		reason := UncertaintyMalformed
		if m.options.Profile != nil && op.signature == renderer.SignatureUnknown {
			reason = UncertaintyUnresolved
			if tagSpec.Behavior == spec.FirstWins {
				for _, slot := range tagSpec.Slots {
					m.uncertainLatches[slot] = true
				}
			}
		}
		m.blockProofs(&event, reason)
		return event
	}
	if tag.InTransition {
		if reason := m.handleTransformChild(tag, tagSpec); reason != UncertaintyNone {
			m.blockProofs(&event, reason)
		}
		return event
	}

	event.Policy = tagSpec.Behavior
	switch tagSpec.Semantic {
	case spec.SemanticStyleReset:
		if !m.resetStyle(tag) {
			m.blockProofs(&event, UncertaintyUnresolved)
		}
		event.Applied = true
		return event
	case spec.SemanticTransform:
		if !m.consumeTransform(tag, index) {
			event.Uncertainty = UncertaintyTimeDependent
		}
		event.Applied = true
		return event
	case spec.SemanticKaraoke:
		if reason := m.accumulate(tag); reason != UncertaintyNone {
			m.blockProofs(&event, reason)
		}
		event.Applied = true
		return event
	}

	slots, behavior := resolveAssignmentPolicy(tag, tagSpec)
	event.Policy = behavior
	event.Slots = slots
	m.fillValues(&event.Before, slots)

	if m.handleRelativeFontSize(tag, tagSpec, &event, slots) {
		return event
	}

	values, valuesKnown := m.assignmentValues(tag, tagSpec, slots)
	event.Known = valuesKnown
	if !valuesKnown {
		// Renderer-dependent or malformed assignments cannot justify
		// removing previously tracked tags, even before this boundary.
		m.blockProofs(&event, assignmentUncertainty(tag, tagSpec))
	}
	if behavior == spec.Assign && valuesKnown && m.sameSlotValues(slots, values) {
		if !m.proofsDisabled {
			m.markCandidate(index, tag, SameValue, -1, true)
		}
		m.fillValues(&event.After, slots)
		return event
	}

	var assignedSlots, assignedValues []string
	for slotIndex, slot := range slots {
		if behavior == spec.FirstWins {
			if m.uncertainLatches[slot] {
				// A prior unresolved command might have claimed this
				// first-wins slot. Later valid commands cannot restore a
				// definite value or a proved owner.
				m.state[slot] = UnknownValue()
				m.active[slot] = owner{index: -1, start: m.position}
				continue
			}
			if first, exists := m.latched[slot]; exists {
				if !m.proofsDisabled && !m.uncertainLatches[slot] {
					m.markCandidate(index, tag, FirstWinsIgnored, first, false)
				}
				continue
			}
			m.latched[slot] = index
			if !valuesKnown {
				m.uncertainLatches[slot] = true
			}
		}
		if previous, exists := m.active[slot]; exists && previous.index >= 0 &&
			previous.start == m.position && previous.proven && !m.proofsDisabled {
			m.markCandidate(previous.index, m.tagAt(previous.index), OverwrittenBeforeUse, index, false)
		}
		m.active[slot] = owner{index: index, start: m.position, proven: valuesKnown}
		assignedSlots = append(assignedSlots, slot)
		if slot == "position" || slot == "origin" {
			m.collisionDisabled = true
		}
		if valuesKnown {
			assignedValues = append(assignedValues, values[slotIndex])
		}
	}
	SetSlotValues(m.state, assignedSlots, assignedValues, valuesKnown)
	event.Applied = len(assignedSlots) > 0
	m.fillValues(&event.After, slots)
	return event
}

func (m *evaluator) value(slot string) StateValue {
	return m.view().Value(slot)
}

func (m *evaluator) sameSlotValues(slots, values []string) bool {
	if len(slots) == 0 || len(slots) != len(values) {
		return false
	}
	for i, slot := range slots {
		value := m.value(slot)
		if !value.Known || value.Value != values[i] {
			return false
		}
	}
	return true
}

func (m *evaluator) fillValues(dst *[4]StateValue, slots []string) {
	if m.options.Observer.Tag == nil {
		return
	}
	for i, slot := range slots {
		if i >= len(dst) {
			break
		}
		dst[i] = m.value(slot)
	}
}

func (m *evaluator) invalidate(slots []string) {
	for _, slot := range slots {
		m.state[slot] = UnknownValue()
		delete(m.active, slot)
	}
}

func (m *evaluator) invalidateAll() {
	m.baseValid = false
	for slot := range m.state {
		m.state[slot] = UnknownValue()
		m.active[slot] = owner{index: -1, start: m.position}
	}
	// These have built-in defaults and must not magically become known again
	// at a later Style reset, because the reset preserves them.
	m.state["drawing_scale"] = UnknownValue()
	m.state["karaoke_cursor"] = UnknownValue()
}

// revokeProofs is a dialogue-wide fence: no prior or subsequent no-effect
// candidate can be guaranteed across all supported renderers after an
// unmodeled operation. Existing candidates remain reportable as diagnostics,
// but carry ProofRevoked so lint cannot emit SafeFix edits from them.
// State invalidation remains a separate concern.
func (m *evaluator) revokeProofs() {
	m.proofsDisabled = true
}

// A semantic barrier revokes earlier and later SafeFix proofs, including
// candidates emitted before the unknown operation was encountered.
func (m *evaluator) blockProofs(event *TagEvent, reason SemanticUncertainty) {
	m.revokeProofs()
	event.Barrier = true
	event.Uncertainty = reason
}

func (m *evaluator) markCandidate(index int, tag ass.Tag, reason NoEffectReason, ownerIndex int, always bool) {
	if previous, exists := m.candidates[index]; exists {
		always = always || previous.always
		if ownerIndex < 0 {
			ownerIndex = previous.ownerIndex
		}
	}
	m.candidates[index] = noEffectCandidate{
		tag: tag, reason: reason, ownerIndex: ownerIndex, always: always,
	}
}

func (m *evaluator) tagAt(index int) ass.Tag {
	if index >= 0 && index < len(m.allTags) {
		return m.allTags[index]
	}
	return ass.Tag{}
}
