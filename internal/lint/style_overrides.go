package lint

import (
	"sort"
	"strconv"
	"strings"

	"assx/internal/ass"
)

const (
	fontNameProperty        = "fontname"
	fontSizeProperty        = "fontsize"
	boldProperty            = "bold"
	italicProperty          = "italic"
	scaleXProperty          = "scalex"
	scaleYProperty          = "scaley"
	spacingProperty         = "spacing"
	underlineProperty       = "underline"
	strikeOutProperty       = "strikeout"
	primaryColourProperty   = "primarycolour"
	secondaryColourProperty = "secondarycolour"
	outlineColourProperty   = "outlinecolour"
	backColourProperty      = "backcolour"
	outlineProperty         = "outline"
	shadowProperty          = "shadow"
	angleProperty           = "angle"
	encodingProperty        = "encoding"
)

var stylePropertySlots = map[string][]string{
	fontNameProperty:        {"fontname"},
	fontSizeProperty:        {"fontsize"},
	boldProperty:            {"bold"},
	italicProperty:          {"italic"},
	scaleXProperty:          {"scale_x"},
	scaleYProperty:          {"scale_y"},
	spacingProperty:         {"spacing"},
	underlineProperty:       {"underline"},
	strikeOutProperty:       {"strikeout"},
	primaryColourProperty:   {"c1", "a1"},
	secondaryColourProperty: {"c2", "a2"},
	outlineColourProperty:   {"c3", "a3"},
	backColourProperty:      {"c4", "a4"},
	outlineProperty:         {"border_x", "border_y"},
	shadowProperty:          {"shadow_x", "shadow_y"},
	angleProperty:           {"frz"},
	encodingProperty:        {"charset"},
}

type styleDefinitionKey struct {
	name string
	line int
}

type overrideBlockRange struct {
	start int
	end   int
}

type styleState struct {
	slots  map[string]bool
	values map[string]stateValue
}

// AnalyzeRedundantStyleOverrides checks Style-backed override state for each dialogue.
func AnalyzeRedundantStyleOverrides(doc ass.Document) []Diagnostic {
	styles := styleDefinitionsByName(doc.StyleFields)
	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		fields, ok := styles[dialogueStyleLookupName(dialogue.Style)]
		if !ok {
			continue
		}
		diagnostic, ok := analyzeRedundantStyleDialogue(dialogue, fields)
		if ok {
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	return diagnostics
}

func styleDefinitionsByName(fields []ass.StyleField) map[string]map[string]string {
	rows := make(map[styleDefinitionKey]map[string]string)
	duplicates := make(map[styleDefinitionKey]bool)
	for _, field := range fields {
		name := strings.TrimSpace(field.StyleName)
		fieldName := strings.ToLower(strings.TrimSpace(field.Name))
		if name == "" || fieldName == "" {
			continue
		}
		key := styleDefinitionKey{name: name, line: field.Line}
		row := rows[key]
		if row == nil {
			row = make(map[string]string)
			rows[key] = row
		}
		if _, exists := row[fieldName]; exists {
			duplicates[key] = true
		}
		row[fieldName] = field.Value
	}

	byName := make(map[string][]styleDefinitionKey)
	for key := range rows {
		byName[key.name] = append(byName[key.name], key)
	}
	styles := make(map[string]map[string]string)
	for name, keys := range byName {
		if len(keys) != 1 || duplicates[keys[0]] {
			continue
		}
		styles[name] = rows[keys[0]]
	}
	return styles
}

func analyzeRedundantStyleDialogue(dialogue ass.Dialogue, styleFields map[string]string) (Diagnostic, bool) {
	base := canonicalStyleState(styleFields)
	state := make(map[string]stateValue, len(base.values))
	for slot, value := range base.values {
		state[slot] = value
	}

	tokens := Lex(dialogue.Text)
	var allTags, styleTags []Tag
	touched := make(map[string]bool)
	hasText := false
	for _, token := range tokens {
		if token.Tag == nil {
			if token.Text == "" {
				continue
			}
			hasText = true
			for slot := range touched {
				if state[slot] != base.values[slot] {
					return Diagnostic{}, false
				}
			}
			continue
		}

		tag := *token.Tag
		allTags = append(allTags, tag)
		if !safeIndependentStyleTag(tag) {
			return Diagnostic{}, false
		}
		spec := TagSpecs[tag.Name]
		var slots []string
		for _, slot := range spec.Slots {
			if base.slots[slot] {
				slots = append(slots, slot)
			}
		}
		if len(slots) == 0 {
			continue
		}
		if len(slots) != len(spec.Slots) {
			return Diagnostic{}, false
		}
		values, ok := styleTagState(tag, spec, slots, base.values)
		if !ok {
			return Diagnostic{}, false
		}
		styleTags = append(styleTags, tag)
		for i, slot := range slots {
			state[slot] = stateValue{value: values[i], known: true}
			touched[slot] = true
		}
	}
	if !hasText || len(styleTags) == 0 {
		return Diagnostic{}, false
	}

	edits := redundantStyleEdits(dialogue, allTags, styleTags)
	if len(edits) == 0 {
		return Diagnostic{}, false
	}
	first := styleTags[0]
	return Diagnostic{
		ID: IssueRedundantStyleOverrides, Severity: Suggestion, FixSafety: SafeFix,
		Line: dialogue.Line, Column: first.Column, Tag: first.Name,
		Detail: "The override tags leave the tracked Style properties unchanged across dialogue text.",
		Edits:  edits,
	}, true
}

func canonicalStyleState(fields map[string]string) styleState {
	state := styleState{slots: make(map[string]bool), values: make(map[string]stateValue)}
	for property, slots := range stylePropertySlots {
		raw, exists := fields[property]
		if !exists {
			continue
		}
		for _, slot := range slots {
			state.slots[slot] = true
		}
		values, ok := canonicalStyleValues(property, raw, slots)
		if !ok {
			continue
		}
		for i, slot := range slots {
			state.values[slot] = stateValue{value: values[i], known: true}
		}
	}
	return state
}

func canonicalStyleValues(property, raw string, slots []string) ([]string, bool) {
	raw = strings.TrimSpace(raw)
	var value string
	var ok bool
	switch property {
	case fontNameProperty:
		value, ok = raw, raw != ""
	case boldProperty:
		value, ok = canonicalStateBold(raw, true)
	case italicProperty, underlineProperty, strikeOutProperty:
		var number int
		number, ok = parseStyleInteger(raw)
		if ok && (number == -1 || number == 1) {
			number = 1
		}
		if ok && (number == 0 || number == 1) {
			value = strconv.Itoa(number)
		} else {
			ok = false
		}
	case fontSizeProperty, scaleXProperty, scaleYProperty, spacingProperty, outlineProperty, shadowProperty, angleProperty:
		if property == scaleXProperty || property == scaleYProperty {
			if number, parsed := parseStateFloat(raw); parsed && number < 0 {
				raw = "0"
			}
		}
		value, ok = canonicalStateNumber(raw)
	case encodingProperty:
		value, ok = canonicalStateInteger(raw)
	case primaryColourProperty, secondaryColourProperty, outlineColourProperty, backColourProperty:
		colour, alpha, valid := canonicalStyleColour(raw)
		if !valid {
			return nil, false
		}
		return []string{colour, alpha}, true
	default:
		return nil, false
	}
	if !ok {
		return nil, false
	}
	values := make([]string, len(slots))
	for i := range values {
		values[i] = value
	}
	return values, true
}

func canonicalStyleColour(raw string) (colour, alpha string, ok bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && strings.EqualFold(raw[:2], "&H") {
		raw = raw[2:]
	}
	if strings.HasSuffix(raw, "&") {
		raw = raw[:len(raw)-1]
	}
	if len(raw) != 8 {
		return "", "", false
	}
	value, err := strconv.ParseUint(raw, 16, 32)
	if err != nil {
		return "", "", false
	}
	return strconv.FormatUint(value&0xffffff, 16), strconv.FormatUint((value>>24)&0xff, 16), true
}

func parseStyleInteger(raw string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	return value, err == nil
}

func styleTagState(tag Tag, spec TagSpec, slots []string, base map[string]stateValue) ([]string, bool) {
	name := strings.ToLower(tag.Name)
	resetToStyle := false
	switch name {
	case "fn":
		resetToStyle = len(tag.Args) == 0 || (!tag.Paren && len(tag.Args) == 1 && strings.TrimSpace(tag.Args[0]) == "0")
	case "fs":
		resetToStyle = len(tag.Args) == 0
		if len(tag.Args) == 1 && !relativeFontSize(tag) {
			if number, ok := parseStateFloat(tag.Args[0]); ok && number == 0 {
				resetToStyle = true
			}
		}
	case "fscx", "fscy", "fsp", "b", "i":
		resetToStyle = len(tag.Args) == 0
	}
	if resetToStyle {
		values := make([]string, len(slots))
		for i, slot := range slots {
			value := base[slot]
			if !value.known {
				return nil, false
			}
			values[i] = value.value
		}
		return values, true
	}
	return canonicalTagState(tag, spec, slots)
}

func safeIndependentStyleTag(tag Tag) bool {
	if tag.InTransition || tag.RepeatedSlashes > 0 || tag.Name == "p" || tag.Name == "n" || tag.Name == "h" || relativeFontSize(tag) {
		return false
	}
	spec, ok := TagSpecs[tag.Name]
	if !ok || spec.VSFilterModOnly || spec.Behavior == StyleReset || spec.Behavior == Transition {
		return false
	}
	if spec.Counts != nil {
		validCount := false
		for _, count := range spec.Counts {
			if count == len(tag.Args) {
				validCount = true
				break
			}
		}
		if !validCount {
			return false
		}
	}
	return true
}

func redundantStyleEdits(dialogue ass.Dialogue, allTags, styleTags []Tag) []TextEdit {
	text := dialogue.Text
	candidate := make(map[[2]int]bool, len(styleTags))
	allByBlock := make(map[overrideBlockRange][]Tag)
	styleByBlock := make(map[overrideBlockRange][]Tag)
	for _, tag := range allTags {
		block, ok := overrideTagBlock(text, tag)
		if !ok {
			continue
		}
		allByBlock[block] = append(allByBlock[block], tag)
	}
	for _, tag := range styleTags {
		candidate[[2]int{tag.Start, tag.End}] = true
		block, ok := overrideTagBlock(text, tag)
		if ok {
			styleByBlock[block] = append(styleByBlock[block], tag)
		}
	}

	var edits []TextEdit
	for block, tags := range styleByBlock {
		all := allByBlock[block]
		if len(all) == len(tags) && onlyWhitespaceBetweenTags(text, block, all, candidate) {
			edits = append(edits, TextEdit{Start: dialogue.TextStart + block.start, End: dialogue.TextStart + block.end})
			continue
		}
		for _, tag := range tags {
			edits = append(edits, TextEdit{Start: dialogue.TextStart + tag.Start, End: dialogue.TextStart + tag.End})
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start < edits[j].Start })
	return edits
}

func overrideTagBlock(text string, tag Tag) (overrideBlockRange, bool) {
	if tag.Start < 0 || tag.End > len(text) || tag.Start >= tag.End {
		return overrideBlockRange{}, false
	}
	open := strings.LastIndex(text[:tag.Start], "{")
	closeOffset := strings.IndexByte(text[tag.End:], '}')
	if open < 0 || closeOffset < 0 {
		return overrideBlockRange{}, false
	}
	close := tag.End + closeOffset
	if open >= tag.Start || close < tag.End {
		return overrideBlockRange{}, false
	}
	return overrideBlockRange{start: open, end: close + 1}, true
}

func onlyWhitespaceBetweenTags(text string, block overrideBlockRange, tags []Tag, candidates map[[2]int]bool) bool {
	sort.Slice(tags, func(i, j int) bool { return tags[i].Start < tags[j].Start })
	cursor := block.start + 1
	for _, tag := range tags {
		if !candidates[[2]int{tag.Start, tag.End}] || tag.Start < cursor || tag.End > block.end-1 {
			return false
		}
		if strings.TrimSpace(text[cursor:tag.Start]) != "" {
			return false
		}
		cursor = tag.End
	}
	return strings.TrimSpace(text[cursor:block.end-1]) == ""
}
