package lint

import (
	"math"
	"strconv"
	"strings"
)

type stateValue struct {
	value string
	known bool
}

func canonicalTagState(tag Tag, spec TagSpec, slots []string) ([]string, bool) {
	if len(slots) == 0 {
		return nil, false
	}
	if tag.Name == "fsc" {
		value, ok := canonicalStateNumber("100")
		if !ok {
			return nil, false
		}
		values := make([]string, len(slots))
		for i := range values {
			values[i] = value
		}
		return values, true
	}
	if len(tag.Args) != 1 {
		return nil, false
	}
	raw := strings.TrimSpace(tag.Args[0])
	var value string
	var ok bool
	switch spec.Value {
	case IntegerValue:
		value, ok = canonicalStateInteger(raw)
	case NumberValue:
		if tag.Name == "fs" {
			number, parsed := parseStateFloat(raw)
			if !parsed || number <= 0 {
				return nil, false
			}
		}
		if tag.Name == "fscx" || tag.Name == "fscy" {
			if number, parsed := parseStateFloat(raw); parsed && number < 0 {
				raw = "0"
			}
		}
		value, ok = canonicalStateNumber(raw)
	case BoldValue:
		value, ok = canonicalStateBold(raw, false)
	case FontNameValue:
		if raw != "" && raw != "0" {
			value, ok = raw, true
		}
	case HexValue:
		parsed, valid := parseStateHex(raw)
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
	case NumberListValue:
		parts := make([]string, len(tag.Args))
		for i, arg := range tag.Args {
			parts[i], ok = canonicalStateNumber(arg)
			if !ok {
				return nil, false
			}
		}
		value, ok = strings.Join(parts, ","), true
	case RectValue:
		if len(tag.Args) == 4 {
			parts := make([]string, len(tag.Args))
			for i, arg := range tag.Args {
				parts[i], ok = canonicalStateNumber(arg)
				if !ok {
					return nil, false
				}
			}
			value, ok = strings.Join(parts, ","), true
		}
	case NoValue:
		if slots[0] == "charset" {
			value, ok = canonicalStateInteger(raw)
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

func relativeFontSize(tag Tag) bool {
	if tag.Name != "fs" || len(tag.Args) == 0 {
		return false
	}
	raw := strings.TrimSpace(tag.Args[0])
	return strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-")
}

func canonicalStateInteger(raw string) (string, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return strconv.Itoa(value), true
}

func canonicalStateBold(raw string, style bool) (string, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || (value != 0 && value != 1 && value < 100 && !(style && value == -1)) {
		return "", false
	}
	if value == -1 || value == 1 {
		value = 1
	}
	return strconv.Itoa(value), true
}

func canonicalStateNumber(raw string) (string, bool) {
	value, ok := parseStateFloat(raw)
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

func parseStateFloat(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func parseStateHex(raw string) (uint64, bool) {
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

func sameSlotValues(state map[string]stateValue, slots, values []string) bool {
	if len(slots) == 0 || len(slots) != len(values) {
		return false
	}
	for i, slot := range slots {
		current := state[slot]
		if !current.known || current.value != values[i] {
			return false
		}
	}
	return true
}

func setSlotValues(state map[string]stateValue, slots, values []string, known bool) {
	for i, slot := range slots {
		value := stateValue{known: known}
		if known && i < len(values) {
			value.value = values[i]
		}
		state[slot] = value
	}
}
