package semantic

import (
	"strconv"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

// Tag-specific semantics live here; simple assignments remain metadata-driven.
// No per-tag interface allocations or runtime registration are required.

func (m *evaluator) handleTransformChild(tag ass.Tag, tagSpec spec.TagSpec) SemanticUncertainty {
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
		return UncertaintyNone
	}
	reason := assignmentUncertainty(tag, tagSpec)
	if reason == UncertaintyMalformed || reason == UncertaintyRendererDependent {
		return reason
	}
	return UncertaintyUnsupported
}

func (m *evaluator) consumeTransform(tag ass.Tag, index int) bool {
	if len(tag.Children) > 0 {
		m.markActiveLive()
	}
	noEffect := m.transitionHasNoEffect(tag)
	if noEffect && m.collisionDisabled && !m.proofsDisabled {
		m.markCandidate(index, tag, TransitionNoEffect, -1, true)
	}
	m.collisionDisabled = true
	if !noEffect {
		m.invalidateAll()
	}
	return noEffect
}

func resolveAssignmentPolicy(tag ass.Tag, tagSpec spec.TagSpec) ([]string, spec.Behavior) {
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

func (m *evaluator) handleRelativeFontSize(tag ass.Tag, tagSpec spec.TagSpec, event *TagEvent, slots []string) bool {
	if tagSpec.Semantic != spec.SemanticFontSize || !RelativeFontSize(tag) {
		return false
	}
	m.invalidate([]string{"fontsize"})
	m.blockProofs(event, UncertaintyUnsupported)
	m.fillValues(&event.After, slots)
	return true
}

// assignmentUncertainty preserves the decoder's distinctions without
// making a renderer-specific assumption when canonicalization fails.
func assignmentUncertainty(tag ass.Tag, tagSpec spec.TagSpec) SemanticUncertainty {
	ir := ass.DecodeTag(tag)
	if tagSpec.Semantic == spec.SemanticClip && len(tag.Args) != 4 && ir.HasExpectedArity() {
		return UncertaintyUnsupported
	}
	switch ir.ArgumentsStatus() {
	case ass.ValueInvalid:
		return UncertaintyMalformed
	case ass.ValueAmbiguous:
		return UncertaintyRendererDependent
	case ass.ValueUnknown:
		return UncertaintyUnresolved
	}
	if tagSpec.Value == spec.IntegerValue && len(tag.Args) == 1 {
		decoded := ir.Argument(0)
		if decoded.Status == ass.ValueValid && (tagSpec.Min != 0 || tagSpec.Max != 0) &&
			(decoded.Integer < int64(tagSpec.Min) || decoded.Integer > int64(tagSpec.Max)) {
			return UncertaintyMalformed
		}
	}
	return UncertaintyUnresolved
}

func (m *evaluator) assignmentValues(tag ass.Tag, tagSpec spec.TagSpec, slots []string) ([]string, bool) {
	var values []string
	var valuesKnown bool
	if style, present := m.options.Styles[m.activeStyle]; present {
		original := m.options.Styles[m.originalStyle]
		values, valuesKnown = StyleTagState(tag, tagSpec, slots, style.Values, original.Values)
	} else {
		values, valuesKnown = CanonicalTagState(tag, tagSpec, slots)
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
	m.state[slot] = UnknownValue()
	m.active[slot] = owner{index: -1, start: m.position}
	return reason
}

func (m *evaluator) accumulate(tag ass.Tag) SemanticUncertainty {
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
		value := ass.DecodeNumber(tag.Args[0])
		// Prefix-only decoding is sufficient for syntax diagnostics, but not
		// for proof of a canonical karaoke duration.
		if value.Status != ass.ValueValid || value.Consumed != len(value.Raw) ||
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
	m.state[slot] = KnownValue(strconv.FormatInt(previous+duration, 10))
	m.active[slot] = owner{index: m.tagIndex - 1, start: m.position, proven: true}
	return UncertaintyNone
}

func (m *evaluator) transitionHasNoEffect(tag ass.Tag) bool {
	if len(tag.Children) == 0 {
		return true
	}
	for _, child := range tag.Children {
		tagSpec := spec.TagSpecs[child.Name]
		if child.RepeatedSlashes > 0 || !tagSpec.TransformComparable || RelativeFontSize(child) {
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

func (m *evaluator) resetStyle(tag ass.Tag) bool {
	target := m.originalStyle
	if len(tag.Args) > 0 && strings.TrimSpace(strings.Join(tag.Args, ",")) != "" {
		target = strings.TrimSpace(strings.Join(tag.Args, ","))
	}
	// Reset clears explicit overrides, exposing the new Style defaults. It
	// cannot clear first-wins slots nor state explicitly kept across resets.
	for slot, previous := range m.active {
		if spec.KeepOnStyleReset[slot] {
			continue
		}
		if _, latched := m.latched[slot]; latched {
			continue
		}
		if previous.index >= 0 && previous.start == m.position && previous.proven && !m.proofsDisabled {
			m.markCandidate(previous.index, m.tagAt(previous.index), ResetBeforeUse, -1, false)
		}
		delete(m.active, slot)
	}
	for slot := range m.state {
		if spec.KeepOnStyleReset[slot] {
			continue
		}
		if _, latched := m.latched[slot]; latched {
			continue
		}
		delete(m.state, slot)
	}
	m.activeStyle = target
	_, known := m.options.Styles[target]
	m.baseValid = len(m.options.Styles) == 0 || known
	// Without a Style table, only the original Dialogue Style can be
	// resolved. An arbitrary named reset must not establish a SafeFix proof.
	return known || (len(m.options.Styles) == 0 && target == m.originalStyle)
}
