package semantic

import (
	"strconv"
	"strings"
	"unicode"

	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

// Tag-specific semantics live here; simple assignments remain metadata-driven.
// No per-tag interface allocations or runtime registration are required.

func (m *evaluator) handleTransformChild(tag renderer.Tag, tagSpec spec.TagSpec) tagTransition {
	if m.profile.RecursiveTransformChildren() {
		// Mod recursively applies all handlers. Do not publish a static value
		// for animated assignments or let a later child retain pos3 depth.
		if tagSpec.Behavior == spec.Assign && len(tagSpec.Slots) != 0 {
			return tagTransition{invalidate: invalidateSlots, slots: tagSpec.Slots, barrier: UncertaintyTimeDependent}
		}
		return tagTransition{invalidate: invalidateEverything, barrier: UncertaintyTimeDependent}
	}
	// A transform child is evaluated by its parent. Unsupported targets or
	// syntax must still revoke proofs created before the parent transform.
	unknown := tagSpec.Behavior != spec.Assign || RelativeFontSize(tag)
	if tagSpec.Semantic == spec.SemanticClip {
		unknown = true
	} else if len(tagSpec.Slots) != 0 && !unknown {
		_, modeled := CanonicalTagState(tag, tagSpec, tagSpec.Slots)
		unknown = !modeled
	}
	if !unknown {
		return tagTransition{}
	}
	reason := assignmentUncertainty(tag, tagSpec)
	if reason == UncertaintyMalformed || reason == UncertaintyRendererDependent {
		return tagTransition{barrier: reason}
	}
	return tagTransition{barrier: UncertaintyUnsupported}
}

func (m *evaluator) consumeTransform(tag renderer.Tag, index int) (bool, tagTransition) {
	var transition tagTransition
	if len(m.children[tag.Start]) > 0 {
		transition.live = true
	}
	noEffect := m.transitionHasNoEffect(tag)
	if noEffect && m.collisionDisabled && !m.proofsDisabled {
		m.markCandidate(index, tag, TransitionNoEffect, -1, true)
	}
	m.collisionDisabled = true
	if !noEffect {
		transition.invalidate = invalidateEverything
	}
	return noEffect, transition
}

func resolveAssignmentPolicy(tag renderer.Tag, tagSpec spec.TagSpec) ([]string, spec.Behavior) {
	slots, behavior := tagSpec.Slots, tagSpec.Behavior
	if tagSpec.Semantic == spec.SemanticClip {
		if len(tag.Args) == 4 {
			slots, behavior = []string{"clip_rect"}, spec.Assign
		} else {
			slots, behavior = []string{"clip_vector"}, spec.FirstWins
		}
	}
	return slots, behavior
}

// applyModPositionDepth retains the accepted depth write despite the pinned
// three-argument position layout defect (RTS.cpp:3530-3539,4238-4241).
func (m *evaluator) applyModPositionDepth(tag renderer.Tag, index int, event TagEvent) TagEvent {
	event.Policy = spec.FirstWins
	event.Slots = []string{"position", "z_depth"}
	m.fillValues(&event.Before, event.Slots)
	if m.store.latch("position") != noLatch {
		event.Ignored = true
		m.fillValues(&event.After, event.Slots)
		return event
	}
	// An unverified signature marks the latch uncertain. Reading the latch
	// check before that write is safe: the answer is the same either way.
	uncertain := m.operationFor(tag).signature != renderer.SignatureVerified
	claimed := m.store.uncertainLatch("position") || uncertain
	transition := tagTransition{live: true, barrier: UncertaintyRendererDependent}
	if uncertain {
		transition.uncertain = []string{"position"}
	}
	if claimed {
		// A preceding extension may have claimed EF_MOVE and blocked depth.
		transition.invalidate, transition.slots = invalidateSlots, event.Slots
	}
	event = m.applyTransition(event, transition)
	if claimed {
		m.fillValues(&event.After, event.Slots)
		return event
	}
	m.store.setLatch("position", index)
	m.collisionDisabled = true
	m.store.set("position", UnknownValue())
	m.store.setOwner("position", owner{index: index, start: m.position})
	depth := UnknownValue()
	decoded := renderer.DecodeExactNumber(tag.Args[2])
	if decoded.Status == renderer.ValueValid && !strings.ContainsFunc(tag.Raw, func(r rune) bool {
		return r > unicode.MaxASCII && unicode.IsSpace(r)
	}) {
		if value, ok := canonicalFloat(decoded.Number * 80); ok {
			depth = KnownValue(value)
		}
	}
	m.store.set("z_depth", depth)
	m.store.setOwner("z_depth", owner{index: index, start: m.position, proven: depth.Known})
	event.Applied = true
	m.fillValues(&event.After, event.Slots)
	return event
}

func (m *evaluator) handleRelativeFontSize(tag renderer.Tag, tagSpec spec.TagSpec, slots []string) (bool, tagTransition) {
	if tagSpec.Semantic != spec.SemanticFontSize || !RelativeFontSize(tag) {
		return false, tagTransition{}
	}
	return true, tagTransition{invalidate: invalidateSlots, slots: []string{"fontsize"}, barrier: UncertaintyUnsupported}
}

// assignmentUncertainty preserves the decoder's distinctions without
// making a renderer-specific assumption when canonicalization fails.
func assignmentUncertainty(tag renderer.Tag, tagSpec spec.TagSpec) SemanticUncertainty {
	ir := renderer.DecodeTagWithSpec(tag, tagSpec, true)
	if tagSpec.Semantic == spec.SemanticClip && len(tag.Args) != 4 && ir.HasExpectedArity() {
		return UncertaintyUnsupported
	}
	switch ir.ArgumentsStatus() {
	case renderer.ValueInvalid:
		return UncertaintyMalformed
	case renderer.ValueAmbiguous:
		return UncertaintyRendererDependent
	case renderer.ValueUnknown:
		return UncertaintyUnresolved
	}
	if tagSpec.Value == spec.IntegerValue && len(tag.Args) == 1 {
		decoded := ir.Argument(0)
		if decoded.Status == renderer.ValueValid && (tagSpec.Min != 0 || tagSpec.Max != 0) &&
			(decoded.Integer < int64(tagSpec.Min) || decoded.Integer > int64(tagSpec.Max)) {
			return UncertaintyMalformed
		}
	}
	return UncertaintyUnresolved
}

func (m *evaluator) assignmentValues(tag renderer.Tag, tagSpec spec.TagSpec, slots []string) ([]string, bool) {
	// The ASS v4+ Style layout has no field for these slots. libass zeroes
	// them on an empty tag, and xy-VSFilter/VSFilterMod restore their Style
	// value, which is the renderer default zero. (libass 425-434/812-824/
	// 531-546; xy 2315-2321/2336-2342/2455-2467; vsm 2915-2925/2939-2944/
	// 3118-3128.)
	if len(tag.Args) == 0 {
		switch tag.Name {
		case "blur", "be", "fax", "fay", "frx", "fry":
			values := make([]string, len(slots))
			for i := range values {
				values[i] = "0"
			}
			return values, true
		}
	}
	var values []string
	var valuesKnown bool
	if style, present := m.options.Styles[m.activeStyle]; present {
		original := m.options.Styles[m.originalStyle]
		values, valuesKnown = StyleTagState(tag, tagSpec, slots, style.Values, original.Values)
	} else {
		values, valuesKnown = CanonicalTagState(tag, tagSpec, slots)
	}
	modReset := tagSpec.VSFilterModOnly && len(tag.Args) == 0
	if tag.Name == "ortho" && len(tag.Args) == 1 {
		decoded := renderer.DecodeExactInteger(tag.Args[0])
		if decoded.Status != renderer.ValueValid || decoded.Integer < -1<<31 || decoded.Integer > 1<<31-1 {
			return nil, false
		}
		modReset = decoded.Integer != 0 && decoded.Integer != 1
	}
	if modReset {
		if raw, known := tag.RawArgument(); !known || strings.ContainsFunc(raw, func(r rune) bool {
			return r > unicode.MaxASCII && unicode.IsSpace(r)
		}) {
			return nil, false
		}
		original := m.options.Styles[m.originalStyle]
		values = make([]string, len(slots))
		for i, slot := range slots {
			value := original.Values[slot]
			if !value.Known {
				return nil, false
			}
			values[i] = value.Value
		}
		valuesKnown = true
	}
	if tagSpec.Semantic == spec.SemanticDrawingMode {
		if number, ok := tag.IntegerArgument(); ok {
			if number > 0 {
				values = []string{strconv.FormatInt(int64(number), 10)}
			} else {
				values = []string{"0"}
			}
			valuesKnown = true
		} else {
			valuesKnown = false
		}
	}
	return values, valuesKnown
}

// Accumulating karaoke tags contribute to a shared timeline. Unknown forms,
// transform children, and renderer-specific absolute \kt semantics poison the
// timeline rather than returning an incorrect known duration.
// Invalidate provenance as well as value: a previously known karaoke owner
// cannot justify SafeFix decisions after the timeline becomes uncertain.
func (m *evaluator) invalidateKaraoke(reason SemanticUncertainty) SemanticUncertainty {
	const slot = "karaoke_cursor"
	m.store.set(slot, UnknownValue())
	m.store.setOwner(slot, owner{index: -1, start: m.position})
	return reason
}

func (m *evaluator) accumulate(tag renderer.Tag) SemanticUncertainty {
	const slot = "karaoke_cursor"
	if tag.Name == "kt" {
		return m.invalidateKaraoke(UncertaintyRendererDependent)
	}
	current := m.value(slot)
	duration := int64(1000)
	if len(tag.Args) > 1 || tag.InTransition {
		return m.invalidateKaraoke(UncertaintyMalformed)
	}
	if len(tag.Args) == 1 {
		value := renderer.DecodeNumber(tag.Args[0])
		// Prefix-only decoding is sufficient for syntax diagnostics, but not
		// for proof of a canonical karaoke duration.
		if value.Status != renderer.ValueValid || value.Consumed != len(value.Raw) ||
			value.Number < 0 || value.Number > float64(int64(^uint64(0)>>1)/10) {
			return m.invalidateKaraoke(UncertaintyMalformed)
		}
		duration = int64(value.Number) * 10
	}
	previous, err := strconv.ParseInt(current.Value, 10, 64)
	if !current.Known || err != nil {
		return m.invalidateKaraoke(UncertaintyUnresolved)
	}
	if duration > int64(^uint64(0)>>1)-previous {
		return m.invalidateKaraoke(UncertaintyMalformed)
	}
	m.store.set(slot, KnownValue(strconv.FormatInt(previous+duration, 10)))
	m.store.setOwner(slot, owner{index: m.tagIndex - 1, start: m.position, proven: true})
	return UncertaintyNone
}

func (m *evaluator) transitionHasNoEffect(tag renderer.Tag) bool {
	if len(m.children[tag.Start]) == 0 {
		return true
	}
	for _, childStart := range m.children[tag.Start] {
		child := m.operations[childStart].tag
		tagSpec, known := m.policyFor(child)
		if !known || child.RepeatedSlashes > 0 || !tagSpec.TransformComparable || RelativeFontSize(child) {
			return false
		}
		slots := tagSpec.Slots
		if tagSpec.Semantic == spec.SemanticClip {
			if len(child.Args) != 4 {
				return false
			}
			slots = []string{"clip_rect"}
		}
		values, known := CanonicalTagState(child, tagSpec, slots)
		if !known || !m.sameSlotValues(slots, values) {
			return false
		}
	}
	return true
}

func (m *evaluator) resetStyle(tag renderer.Tag) bool {
	target := m.originalStyle
	if len(tag.Args) > 0 && strings.TrimSpace(tag.Args[0]) != "" {
		target = strings.TrimSpace(tag.Args[0])
	}
	// Reset clears explicit overrides, exposing the new Style defaults. It
	// cannot clear first-wins slots nor state explicitly kept across resets.
	var dropValues, dropOwners []string
	m.store.eachOwner(func(slot string, previous owner) {
		if spec.KeepOnStyleReset[slot] || m.store.latch(slot) != noLatch {
			return
		}
		if previous.index >= 0 && previous.start == m.position && previous.proven && !m.proofsDisabled {
			m.markCandidate(previous.index, m.tagAt(previous.index), ResetBeforeUse, -1, false)
		}
		dropOwners = append(dropOwners, slot)
	})
	m.store.eachPresent(func(slot string) {
		if spec.KeepOnStyleReset[slot] || m.store.latch(slot) != noLatch {
			return
		}
		dropValues = append(dropValues, slot)
	})
	for _, slot := range dropOwners {
		m.store.dropOwner(slot)
	}
	for _, slot := range dropValues {
		m.store.delete(slot)
	}
	m.activeStyle = target
	_, known := m.options.Styles[target]
	m.baseValid = len(m.options.Styles) == 0 || known
	// Without a Style table, only the original Dialogue Style can be
	// resolved. An arbitrary named reset must not establish a SafeFix proof.
	return known || (len(m.options.Styles) == 0 && target == m.originalStyle)
}

// invalidateMalformedTag clears every possible affected state slot when the
// declared arity cannot identify which specialized form the renderer sees.
func (m *evaluator) invalidateMalformedTag(tagSpec spec.TagSpec, index int) {
	if tagSpec.Semantic != spec.SemanticClip {
		m.invalidate(tagSpec.Slots)
		return
	}
	// A malformed clip may be interpreted as either a rectangle or a vector.
	// Neither previous value nor its source remains a valid proof.
	m.invalidate([]string{"clip_rect", "clip_vector"})
	// Unknown clip shape prevents a proof about the vector latch, but does
	// not create a first-wins owner for a tag with rejected arguments.
	m.store.markUncertain("clip_vector")
}
