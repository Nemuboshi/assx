package semantic

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"assx/internal/ass"
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
	Tag        ass.Tag
	Reason     NoEffectReason
	OwnerIndex int
}

type owner struct {
	index int
	start int
}

type noEffectCandidate struct {
	tag        ass.Tag
	reason     NoEffectReason
	ownerIndex int
	always     bool
}

type effectiveStateEngine struct {
	position          int
	drawing           bool
	collisionDisabled bool
	tagIndex          int
	active            map[string]owner
	state             map[string]StateValue
	latched           map[string]int
	allTags           []ass.Tag
	live              map[int]bool
	candidates        map[int]noEffectCandidate
}

func EvaluateDialogue(tree ass.DialogueText) []NoEffect {
	engine := effectiveStateEngine{
		active:     make(map[string]owner),
		state:      make(map[string]StateValue),
		latched:    make(map[string]int),
		live:       make(map[int]bool),
		candidates: make(map[int]noEffectCandidate),
	}

	for _, token := range tree.Tokens() {
		if token.Tag == nil {
			engine.consumeText(token.Text)
			continue
		}
		engine.consumeTag(*token.Tag)
	}

	var out []NoEffect
	for index := range engine.allTags {
		candidate, ok := engine.candidates[index]
		if !ok || (!candidate.always && engine.live[index]) {
			continue
		}
		out = append(out, NoEffect{
			Tag:        candidate.tag,
			Reason:     candidate.reason,
			OwnerIndex: candidate.ownerIndex,
		})
	}
	return out
}

func (m *effectiveStateEngine) consumeText(text string) {
	if text == "" {
		return
	}
	if m.drawing {
		m.markActiveLive()
		m.position++
		return
	}
	for i := 0; i < len(text); {
		if text[i] == '\\' && i+1 < len(text) && strings.ContainsRune("Nnh{}", rune(text[i+1])) {
			m.markActiveLive()
			m.position++
			i += 2
			continue
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		m.markActiveLive()
		m.position++
		i += size
	}
}

func (m *effectiveStateEngine) markActiveLive() {
	for _, active := range m.active {
		if active.index >= 0 {
			m.live[active.index] = true
		}
	}
}

func (m *effectiveStateEngine) consumeTag(tag ass.Tag) {
	index := m.tagIndex
	m.tagIndex++
	m.allTags = append(m.allTags, tag)

	if tag.RepeatedSlashes > 0 {
		return
	}

	tagSpec, known := spec.TagSpecs[tag.Name]
	if !known || tag.Name == "N" || tag.Name == "n" || tag.Name == "h" || tagSpec.VSFilterModOnly {
		return
	}
	if tagSpec.Counts != nil && !containsCount(tagSpec.Counts, len(tag.Args)) {
		return
	}
	if tag.InTransition {
		return
	}
	if tagSpec.Behavior == spec.StyleReset {
		m.resetStyle()
		return
	}
	if tagSpec.Behavior == spec.Transition {
		if len(tag.Children) > 0 {
			m.markActiveLive()
		}
		stateNoEffect := m.transitionHasNoEffect(tag)
		if stateNoEffect && m.collisionDisabled {
			m.markCandidate(index, tag, TransitionNoEffect, -1, true)
		}
		m.collisionDisabled = true
		if !stateNoEffect {
			m.invalidateStateAfterTransition()
		}
		return
	}
	if tagSpec.Behavior == spec.Accumulate {
		return
	}

	slots, behavior := tagSpec.Slots, tagSpec.Behavior
	if tag.Name == "clip" || tag.Name == "iclip" {
		if len(tag.Args) == 4 {
			slots, behavior = []string{"clip_rect"}, spec.Assign
		} else {
			slots, behavior = []string{"clip_vector"}, spec.FirstWins
		}
	}

	if RelativeFontSize(tag) {
		m.state["fontsize"] = UnknownValue()
		return
	}

	values, valuesKnown := CanonicalTagState(tag, tagSpec, slots)
	if behavior == spec.Assign && valuesKnown && SameSlotValues(m.state, slots, values) {
		m.markCandidate(index, tag, SameValue, -1, true)
		return
	}

	var assignedSlots, assignedValues []string
	for slotIndex, slot := range slots {
		if behavior == spec.FirstWins {
			if first, exists := m.latched[slot]; exists {
				m.markCandidate(index, tag, FirstWinsIgnored, first, false)
				continue
			}
			m.latched[slot] = index
		}

		if previous, exists := m.active[slot]; exists && previous.index >= 0 && previous.start == m.position {
			m.markCandidate(previous.index, m.tagAt(previous.index), OverwrittenBeforeUse, index, false)
		}

		m.active[slot] = owner{index: index, start: m.position}
		assignedSlots = append(assignedSlots, slot)
		if valuesKnown {
			assignedValues = append(assignedValues, values[slotIndex])
		}
	}

	SetSlotValues(m.state, assignedSlots, assignedValues, valuesKnown)
	for _, slot := range assignedSlots {
		switch slot {
		case "position", "origin":
			m.collisionDisabled = true
		case "drawing_scale":
			value := 0
			if valuesKnown && len(tag.Args) > 0 {
				value, _ = strconv.Atoi(strings.TrimSpace(tag.Args[0]))
			}
			m.drawing = value > 0
		}
	}
}

func (m *effectiveStateEngine) transitionHasNoEffect(tag ass.Tag) bool {
	if len(tag.Children) == 0 {
		return true
	}

	for _, child := range tag.Children {
		if child.RepeatedSlashes > 0 || !safeTransformStateTag(child.Name) || RelativeFontSize(child) {
			return false
		}
		tagSpec := spec.TagSpecs[child.Name]
		slots := tagSpec.Slots
		if child.Name == "clip" || child.Name == "iclip" {
			if len(child.Args) != 4 {
				return false
			}
			slots = []string{"clip_rect"}
		}
		values, known := CanonicalTagState(child, tagSpec, slots)
		if !known || !SameSlotValues(m.state, slots, values) {
			return false
		}
	}
	return true
}

func (m *effectiveStateEngine) invalidateStateAfterTransition() {
	m.state = make(map[string]StateValue)
}

func safeTransformStateTag(name string) bool {
	switch name {
	case "fs", "fscx", "fscy", "fsp",
		"frx", "fry", "frz", "fr", "fax", "fay",
		"bord", "xbord", "ybord", "shad", "xshad", "yshad",
		"blur", "be",
		"c", "1c", "2c", "3c", "4c",
		"alpha", "1a", "2a", "3a", "4a",
		"clip", "iclip":
		return true
	default:
		return false
	}
}

func (m *effectiveStateEngine) markCandidate(index int, tag ass.Tag, reason NoEffectReason, ownerIndex int, always bool) {
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

func (m *effectiveStateEngine) tagAt(index int) ass.Tag {
	if index >= 0 && index < len(m.allTags) {
		return m.allTags[index]
	}
	return ass.Tag{}
}

func (m *effectiveStateEngine) resetStyle() {
	for slot, previous := range m.active {
		if spec.KeepOnStyleReset[slot] {
			continue
		}
		m.state[slot] = UnknownValue()
		if _, firstWins := m.latched[slot]; firstWins {
			continue
		}
		if previous.index >= 0 && previous.start == m.position {
			m.markCandidate(previous.index, m.tagAt(previous.index), ResetBeforeUse, -1, false)
		}
		m.active[slot] = owner{index: -1, start: m.position}
	}
}

func containsCount(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
