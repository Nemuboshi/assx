package semantic

import (
	"math"
	"strconv"
	"strings"

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
	if tag.Name == "fsc" {
		value, ok := CanonicalNumber("100")
		if !ok {
			return nil, false
		}
		values := make([]string, len(slots))
		for i := range values {
			values[i] = value
		}
		return values, true
	}
	var value string
	var ok bool
	switch tagSpec.Value {
	case spec.IntegerValue, spec.NumberValue, spec.BoldValue, spec.FontNameValue, spec.HexValue, spec.NoValue:
		if len(tag.Args) != 1 {
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
			if tag.Name == "fscx" || tag.Name == "fscy" {
				if number, parsed := ParseFloat(raw); parsed && number < 0 {
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
			parsed, valid := ParseHex(raw)
			if valid {
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
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return strconv.Itoa(value), true
}

func CanonicalBold(raw string, style bool) (string, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || (value != 0 && value != 1 && value < 100 && !(style && value == -1)) {
		return "", false
	}
	if value == -1 || value == 1 {
		value = 1
	}
	return strconv.Itoa(value), true
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
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func ParseHex(raw string) (uint64, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && strings.EqualFold(raw[:2], "&H") {
		raw = raw[2:]
	}
	if strings.HasSuffix(raw, "&") {
		raw = raw[:len(raw)-1]
	}
	if raw == "" || len(raw) > 8 {
		return 0, false
	}
	value, err := strconv.ParseUint(raw, 16, 32)
	return value, err == nil
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
