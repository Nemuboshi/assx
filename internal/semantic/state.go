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

// CanonicalTagState decodes every argument at most once. Syntax diagnostics
// may accept a renderer-consumed prefix, but equivalence proofs require an
// unambiguous value whose entire argument is consumed.
func CanonicalTagState(tag ass.Tag, tagSpec spec.TagSpec, slots []string) ([]string, bool) {
	if len(slots) == 0 {
		return nil, false
	}
	ir := ass.DecodeTag(tag)
	var value string
	var ok bool
	switch tagSpec.Value {
	case spec.IntegerValue, spec.NumberValue, spec.BoldValue, spec.FontNameValue, spec.HexValue, spec.NoValue:
		if len(tag.Args) != 1 {
			return nil, false
		}
		// AST arguments trim Unicode whitespace, but renderers do not. Their
		// interpretation of a prefixed non-ASCII space can differ.
		if source, known := tag.RawArgument(); !known || strings.ContainsFunc(source, func(r rune) bool {
			return r != ' ' && r != '\t' && unicode.IsSpace(r)
		}) {
			return nil, false
		}
		raw := strings.TrimSpace(tag.Args[0])
		decoded := ir.Argument(0)
		if decoded.Status != ass.ValueValid || decoded.Consumed != len(decoded.Raw) {
			return nil, false
		}
		switch tagSpec.Value {
		case spec.IntegerValue:
			if (tagSpec.Min != 0 || tagSpec.Max != 0) &&
				(decoded.Integer < int64(tagSpec.Min) || decoded.Integer > int64(tagSpec.Max)) {
				return nil, false
			}
			value = strconv.FormatInt(decoded.Integer, 10)
			ok = true
		case spec.NumberValue:
			number := decoded.Number
			if tag.Name == "fs" && number <= 0 {
				return nil, false
			}
			if (tag.Name == "fscx" || tag.Name == "fscy" || tag.Name == "shad") && number < 0 {
				if tag.Name == "shad" && tag.InTransition {
					return nil, false
				}
				number = 0
			}
			value, ok = canonicalFloat(number)
		case spec.BoldValue:
			value, ok = canonicalBoldInteger(decoded.Integer, false)
		case spec.FontNameValue:
			if raw != "" && raw != "0" {
				value, ok = raw, true
			}
		case spec.HexValue:
			if len(raw) >= 2 && strings.EqualFold(raw[:2], "&H") &&
				(tag.Paren || raw[:2] != "&H") {
				return nil, false
			}
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
		case spec.NoValue:
			if slots[0] == "charset" {
				value, ok = strconv.FormatInt(decoded.Integer, 10), true
			}
		}
	case spec.NumberListValue, spec.RectValue:
		if len(tag.Args) == 0 || (tagSpec.Value == spec.RectValue && len(tag.Args) != 4) {
			return nil, false
		}
		parts := make([]string, len(tag.Args))
		for i := range tag.Args {
			decoded := ir.Argument(i)
			if decoded.Status != ass.ValueValid || decoded.Consumed != len(decoded.Raw) {
				return nil, false
			}
			parts[i], ok = canonicalFloat(decoded.Number)
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

func canonicalFloat(value float64) (string, bool) {
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
	return strconv.FormatFloat(value, 'g', -1, 64) + ":" +
		strconv.FormatFloat(float64(value32), 'g', -1, 32), true
}

func canonicalBoldInteger(value int64, style bool) (string, bool) {
	if value != 0 && value != 1 && value < 100 && !(style && value == -1) {
		return "", false
	}
	if value == -1 || value == 1 {
		value = 1
	}
	return strconv.FormatInt(value, 10), true
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
	return canonicalBoldInteger(decoded.Integer, style)
}

func CanonicalNumber(raw string) (string, bool) {
	value, ok := ParseFloat(raw)
	if !ok {
		return "", false
	}
	return canonicalFloat(value)
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
