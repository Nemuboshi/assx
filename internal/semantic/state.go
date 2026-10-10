package semantic

import (
	"math"
	"strconv"
	"strings"
	"unicode"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

type StateValue struct {
	Value string
	Known bool
}

func UnknownValue() StateValue {
	return StateValue{}
}

func KnownValue(value string) StateValue {
	return StateValue{Value: value, Known: true}
}

func CanonicalTagState(tag ass.Tag, tagSpec spec.TagSpec, slots []string) ([]string, bool) {
	if len(slots) == 0 {
		return nil, false
	}
	var value string
	var ok bool
	decodedTag := ass.DecodeTag(tag)
	// Renderer-dependent values cannot prove equivalence for automatic fixes.
	for i := range tag.Args {
		if decodedTag.Argument(i).Status == ass.ValueAmbiguous {
			return nil, false
		}
	}
	switch tagSpec.Value {
	case spec.IntegerValue, spec.NumberValue, spec.BoldValue, spec.FontNameValue, spec.HexValue, spec.NoValue:
		if len(tag.Args) != 1 {
			return nil, false
		}
		// Renderers only skip ASCII spaces and tabs, but the parser trims Unicode
		// whitespace too, so a form like `\i\u30001` would otherwise read as "1"
		// while both renderers read zero.
		if source, known := tag.RawArgument(); !known || strings.ContainsFunc(source, func(r rune) bool {
			return r != ' ' && r != '\t' && unicode.IsSpace(r)
		}) {
			return nil, false
		}
		raw := strings.TrimSpace(tag.Args[0])
		switch tagSpec.Value {
		case spec.IntegerValue:
			value, ok = CanonicalInteger(raw)
		case spec.NumberValue:
			if tag.Name == "fs" {
				number, parsed := ParseFloat(raw)
				if !parsed || number <= 0 {
					return nil, false
				}
			}
			if tag.Name == "fscx" || tag.Name == "fscy" || tag.Name == "shad" {
				if number, parsed := ParseFloat(raw); parsed && number < 0 {
					if tag.Name == "shad" && tag.InTransition {
						return nil, false
					}
					raw = "0"
				}
			}
			value, ok = CanonicalNumber(raw)
		case spec.BoldValue:
			value, ok = CanonicalBold(raw, false)
		case spec.FontNameValue:
			if raw != "" && raw != "0" {
				value, ok = raw, true
			}
		case spec.HexValue:
			// Prefix handling is shared only for concatenated uppercase &H syntax.
			if len(raw) >= 2 && strings.EqualFold(raw[:2], "&H") && (tag.Paren || raw[:2] != "&H") {
				return nil, false
			}
			decoded := decodedTag.Argument(0)
			if decoded.Status == ass.ValueValid && decoded.Consumed == len(raw) {
				parsed := uint64(decoded.Hex)
				values := make([]string, len(slots))
				for i, slot := range slots {
					if strings.HasPrefix(slot, "a") {
						values[i] = strconv.FormatUint(parsed&0xff, 16)
					} else {
						values[i] = strconv.FormatUint(parsed&0xffffff, 16)
					}
				}
				return values, true
			}
		case spec.NoValue:
			if slots[0] == "charset" {
				value, ok = CanonicalInteger(raw)
			}
		}
	case spec.NumberListValue, spec.RectValue:
		if len(tag.Args) == 0 || (tagSpec.Value == spec.RectValue && len(tag.Args) != 4) {
			return nil, false
		}
		parts := make([]string, len(tag.Args))
		for i, arg := range tag.Args {
			parts[i], ok = CanonicalNumber(arg)
			if !ok {
				return nil, false
			}
		}
		value, ok = strings.Join(parts, ","), true
		if tagSpec.Value == spec.RectValue {
			value = tag.Name + ":" + value
		}
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

func RelativeFontSize(tag ass.Tag) bool {
	if tag.Name != "fs" || len(tag.Args) == 0 {
		return false
	}
	raw := strings.TrimSpace(tag.Args[0])
	return strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-")
}

func CanonicalInteger(raw string) (string, bool) {
	value := ass.DecodeExactInteger(raw)
	if value.Status != ass.ValueValid {
		return "", false
	}
	return strconv.FormatInt(value.Integer, 10), true
}

func CanonicalBold(raw string, style bool) (string, bool) {
	decoded := ass.DecodeExactInteger(raw)
	if decoded.Status != ass.ValueValid {
		return "", false
	}
	value := decoded.Integer
	if value != 0 && value != 1 && value < 100 && !(style && value == -1) {
		return "", false
	}
	if value == -1 || value == 1 {
		value = 1
	}
	return strconv.FormatInt(value, 10), true
}

func CanonicalNumber(raw string) (string, bool) {
	value, ok := ParseFloat(raw)
	if !ok {
		return "", false
	}
	value32 := float32(value)
	if math.IsInf(float64(value32), 0) {
		return "", false
	}
	if value == 0 {
		value = 0
	}
	if value32 == 0 {
		value32 = 0
	}
	return strconv.FormatFloat(value, 'g', -1, 64) + ":" + strconv.FormatFloat(float64(value32), 'g', -1, 32), true
}

func ParseFloat(raw string) (float64, bool) {
	value := ass.DecodeExactNumber(raw)
	return value.Number, value.Status == ass.ValueValid
}

func ParseHex(raw string) (uint64, bool) {
	value := ass.DecodeExactHex(raw)
	return uint64(value.Hex), value.Status == ass.ValueValid
}

func SameSlotValues(state map[string]StateValue, slots, values []string) bool {
	if len(slots) == 0 || len(slots) != len(values) {
		return false
	}
	for i, slot := range slots {
		current := state[slot]
		if !current.Known || current.Value != values[i] {
			return false
		}
	}
	return true
}

func SetSlotValues(state map[string]StateValue, slots, values []string, known bool) {
	for i, slot := range slots {
		value := StateValue{Known: known}
		if known && i < len(values) {
			value.Value = values[i]
		}
		state[slot] = value
	}
}
