package semantic

import (
	"slices"
	"strconv"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
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

type StyleState struct {
	Slots  map[string]bool
	Values map[string]StateValue
}

func DefinedStyleNames(fields []ass.StyleField) map[string]struct{} {
	styles := make(map[string]struct{})
	for _, field := range fields {
		if field.Name == "name" {
			styles[field.Value] = struct{}{}
		}
	}
	return styles
}

func DialogueStyleLookupName(name string) string {
	name = strings.TrimLeft(strings.TrimSpace(name), "*")
	if strings.EqualFold(name, "Default") {
		return "Default"
	}
	return name
}

func ResolveDialogueStyleReference(name string, styles map[string]struct{}) bool {
	_, exists := styles[DialogueStyleLookupName(name)]
	return exists
}

func ResolveResetStyleReference(name string, styles map[string]struct{}) bool {
	_, exists := styles[strings.TrimSpace(name)]
	return exists
}

func StyleDefinitionsByName(fields []ass.StyleField) map[string]map[string]string {
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

func CanonicalStyleState(fields map[string]string) StyleState {
	state := StyleState{Slots: make(map[string]bool), Values: make(map[string]StateValue)}
	for property, slots := range stylePropertySlots {
		raw, exists := fields[property]
		if !exists {
			continue
		}
		for _, slot := range slots {
			state.Slots[slot] = true
		}
		values, ok := canonicalStyleValues(property, raw, slots)
		if !ok {
			continue
		}
		for i, slot := range slots {
			state.Values[slot] = KnownValue(values[i])
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
		value, ok = CanonicalBold(raw, true)
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
			if number, parsed := ParseFloat(raw); parsed && number < 0 {
				raw = "0"
			}
		}
		value, ok = CanonicalNumber(raw)
	case encodingProperty:
		value, ok = CanonicalInteger(raw)
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
	digits := raw
	if len(digits) >= 2 && strings.EqualFold(digits[:2], "&H") {
		digits = digits[2:]
	}
	if strings.HasSuffix(digits, "&") {
		digits = digits[:len(digits)-1]
	}
	if len(digits) != 8 {
		return "", "", false
	}
	parsed := ass.DecodeExactHex(raw)
	if parsed.Status != ass.ValueValid {
		return "", "", false
	}
	value := uint64(parsed.Hex)
	return strconv.FormatUint(value&0xffffff, 16), strconv.FormatUint((value>>24)&0xff, 16), true
}

func parseStyleInteger(raw string) (int, bool) {
	value := ass.DecodeExactInteger(raw)
	return int(value.Integer), value.Status == ass.ValueValid
}

func StyleTagState(tag ass.Tag, tagSpec spec.TagSpec, slots []string, base, original map[string]StateValue) ([]string, bool) {
	name := strings.ToLower(tag.Name)
	resetToStyle := false
	switch name {
	case "fn":
		raw, known := tag.RawArgument()
		if !known {
			return nil, false
		}
		if tag.Paren {
			// Parenthesized arguments drop leading and trailing spaces.
			raw = strings.Trim(raw, " \t\r\n\v\f")
		} else {
			// libass (rskip_spaces) and VSFilter trim trailing spaces, so only
			// leading spaces survive and select the literal family "0".
			raw = strings.TrimRight(raw, " \t\r\n\v\f")
		}
		switch {
		case raw == "", raw == "0":
			resetToStyle = true
		case strings.TrimLeft(raw, " \t\r\n\v\f") == "0":
			return []string{"0"}, true
		}
	case "fs":
		resetToStyle = len(tag.Args) == 0
		if len(tag.Args) == 1 && !RelativeFontSize(tag) {
			if number, ok := ParseFloat(tag.Args[0]); ok && number == 0 {
				resetToStyle = true
			}
		}
	case "fsc":
		// libass and xy-VSFilter reset both scales to the Style values and
		// ignore arguments; only VSFilterMod consumes \fsc(value), so the
		// argument form is not a provable style-backed assignment.
		if len(tag.Args) == 0 {
			resetToStyle = true
		} else {
			return nil, false
		}
	case "b", "i":
		raw, known := tag.RawArgument()
		if !known {
			return nil, false
		}
		// Both renderers skip surrounding whitespace, and an empty argument
		// restores the Style value.
		raw = strings.Trim(raw, " \t\r\n\v\f")
		value, known := tag.IntegerArgument()
		if !known {
			return nil, false
		}
		resetToStyle = raw == "" || (value != 0 && value != 1 && (name == "i" || value < 100))
		if !resetToStyle {
			tag.Args = []string{strconv.FormatInt(int64(value), 10)}
		}
	case "fscx", "fscy", "fsp":
		resetToStyle = len(tag.Args) == 0
	}
	if resetToStyle {
		values := make([]string, len(slots))
		for i, slot := range slots {
			value := base[slot]
			// Named resets change libass's defaults, while VSFilter retains the Dialogue defaults.
			if !value.Known || value != original[slot] {
				return nil, false
			}
			values[i] = value.Value
		}
		return values, true
	}
	return CanonicalTagState(tag, tagSpec, slots)
}

func SafeIndependentStyleTag(tag ass.Tag) bool {
	if tag.InTransition || tag.RepeatedSlashes > 0 || tag.Name == "p" || tag.Name == "n" || tag.Name == "h" || RelativeFontSize(tag) {
		return false
	}
	tagSpec, ok := spec.TagSpecs[tag.Name]
	if !ok || tagSpec.VSFilterModOnly || tagSpec.Behavior == spec.StyleReset || tagSpec.Behavior == spec.Transition {
		return false
	}
	if tagSpec.Counts != nil && !slices.Contains(tagSpec.Counts, len(tag.Args)) {
		return false
	}
	return true
}
