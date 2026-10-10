package ass

import (
	"math"
	"slices"
	"strconv"
	"strings"

	"assx/internal/ass/spec"
)

// ValueStatus separates a usable value from malformed input, values whose
// meaning cannot be established, and renderer-dependent interpretations.
type ValueStatus uint8

const (
	ValueInvalid ValueStatus = iota
	ValueValid
	ValueUnknown
	ValueAmbiguous
)

// DecodedArgument is a transient typed reading of an AST argument.
// Consumed counts bytes in Raw, including any leading ASCII whitespace.
// The original Tag.Raw and source spans remain the authority for edits.
type DecodedArgument struct {
	Raw      string
	Kind     spec.ValueKind
	Status   ValueStatus
	Consumed int
	Number   float64
	Integer  int64
	Hex      uint32
}

// Normalized returns a canonical scalar spelling only for an unambiguous,
// completely consumed value. Renderer-aware state canonicalization (including
// float32 rounding and resets) belongs to the semantic layer.
func (arg DecodedArgument) Normalized() (string, bool) {
	if arg.Status != ValueValid || arg.Consumed != len(arg.Raw) {
		return "", false
	}
	switch arg.Kind {
	case spec.IntegerValue, spec.BoldValue:
		return strconv.FormatInt(arg.Integer, 10), true
	case spec.NumberValue:
		return strconv.FormatFloat(arg.Number, 'g', -1, 64), true
	case spec.HexValue:
		return strconv.FormatUint(uint64(arg.Hex), 16), true
	case spec.FontNameValue:
		return arg.Raw, true
	default:
		return "", false
	}
}

// TagIR is a small, allocation-free view over the lossless syntax tree.
// Decoding is on demand; no second copy of decoded arguments is stored in AST.
type TagIR struct {
	Tag   Tag
	Spec  spec.TagSpec
	Known bool
}

func DecodeTag(tag Tag) TagIR {
	s, ok := spec.TagSpecs[tag.Name]
	return DecodeTagWithSpec(tag, s, ok)
}

// DecodeTagWithSpec decodes an already resolved command. The caller owns the
// renderer-specific policy; lexical decoding never selects a renderer.
func DecodeTagWithSpec(tag Tag, policy spec.TagSpec, recognized bool) TagIR {
	return TagIR{Tag: tag, Spec: policy, Known: recognized}
}

// HasExpectedArity checks the declared structured argument signature.
func (ir TagIR) HasExpectedArity() bool {
	return ir.Known && (ir.Spec.Counts == nil || slices.Contains(ir.Spec.Counts, len(ir.Tag.Args)))
}

// ArgumentsStatus reports the combined shape and lexical status without
// allocating a decoded slice. A partially consumed value is unknown for
// semantics even when it has a usable prefix for lint diagnostics.
func (ir TagIR) ArgumentsStatus() ValueStatus {
	if !ir.Known {
		return ValueUnknown
	}
	if !ir.HasExpectedArity() {
		return ValueInvalid
	}
	if len(ir.Tag.Args) == 0 || (ir.Spec.Value == spec.RectValue && len(ir.Tag.Args) != 4) {
		return ValueUnknown // Empty, vector-clip and reset forms need specialized semantics.
	}
	status := ValueValid
	for i := range ir.Tag.Args {
		part := ir.Argument(i)
		if part.Status == ValueInvalid {
			return ValueInvalid
		}
		if part.Status == ValueUnknown || part.Consumed != len(part.Raw) {
			return ValueUnknown
		}
		if part.Status == ValueAmbiguous {
			status = ValueAmbiguous
		}
	}
	return status
}

// Argument decodes one already-tokenized argument using TagSpec metadata.
// Structured tags use the same scalar decoder per component. Special syntax,
// such as transforms, resets and vector clips, remains in the semantic layer.
func (ir TagIR) Argument(index int) DecodedArgument {
	if !ir.Known || index < 0 || index >= len(ir.Tag.Args) {
		return DecodedArgument{Status: ValueUnknown}
	}
	raw := ir.Tag.Args[index]
	var value DecodedArgument
	switch ir.Spec.Value {
	case spec.IntegerValue, spec.BoldValue:
		value = DecodeInteger(raw)
	case spec.NumberValue, spec.NumberListValue, spec.RectValue:
		value = DecodeNumber(raw)
	case spec.HexValue:
		value = DecodeHex(raw)
	case spec.FontNameValue:
		value = DecodedArgument{Raw: raw, Kind: spec.FontNameValue, Status: ValueValid, Consumed: len(raw)}
	case spec.NoValue:
		if ir.Tag.Name == "fe" {
			value = DecodeInteger(raw)
		} else {
			value = DecodedArgument{Raw: raw, Kind: spec.NoValue, Status: ValueUnknown}
		}
	}
	if value.Status != ValueValid {
		return value
	}
	switch {
	case ir.Spec.Value == spec.HexValue && hasHexPrefix(strings.TrimSpace(raw)) &&
		(ir.Tag.Paren || !strings.HasPrefix(strings.TrimSpace(raw), "&H")):
		value.Status = ValueAmbiguous
	case ir.Tag.Name == "a" && (value.Integer == 4 || value.Integer == 8):
		value.Status = ValueAmbiguous
	case ir.Tag.Name == "blur" && value.Number > 100:
		value.Status = ValueAmbiguous
	case (ir.Tag.Name == "clip" || ir.Tag.Name == "iclip") && len(ir.Tag.Args) == 4 &&
		math.Trunc(value.Number) != math.Trunc(value.Number+0.5):
		value.Status = ValueAmbiguous
	}
	return value
}

// DecodeInteger consumes a signed decimal integer prefix with the ASCII
// whitespace grammar used by the lint parser. Overflow is unknown instead
// of silently clamping or wrapping.
func DecodeInteger(raw string) DecodedArgument {
	result := DecodedArgument{Raw: raw, Kind: spec.IntegerValue, Status: ValueInvalid}
	end, ok := integerEnd(raw, false)
	if !ok {
		return result
	}
	result.Consumed = end
	begin := skipNumericSpace(raw, false)
	value, err := strconv.ParseInt(raw[begin:end], 10, 64)
	if err != nil {
		result.Status = ValueUnknown
		return result
	}
	result.Integer = value
	result.Status = ValueValid
	return result
}

// DecodeNumber accepts the numeric prefix syntax without swallowing trailing
// junk. This matches the lexical range recognized by lint, including a
// properly formed optional exponent.
func DecodeNumber(raw string) DecodedArgument {
	result := DecodedArgument{Raw: raw, Kind: spec.NumberValue, Status: ValueInvalid}
	start := skipNumericSpace(raw, false)
	pos := start
	if pos < len(raw) && (raw[pos] == '+' || raw[pos] == '-') {
		pos++
	}
	digits := 0
	for pos < len(raw) && decimalDigit(raw[pos]) {
		pos++
		digits++
	}
	if pos < len(raw) && raw[pos] == '.' {
		pos++
		for pos < len(raw) && decimalDigit(raw[pos]) {
			pos++
			digits++
		}
	}
	if digits == 0 {
		return result
	}
	if pos < len(raw) && (raw[pos] == 'e' || raw[pos] == 'E') {
		exp := pos + 1
		if exp < len(raw) && (raw[exp] == '+' || raw[exp] == '-') {
			exp++
		}
		first := exp
		for exp < len(raw) && decimalDigit(raw[exp]) {
			exp++
		}
		if exp > first {
			pos = exp
		}
	}
	result.Consumed = pos
	value, err := strconv.ParseFloat(raw[start:pos], 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		result.Status = ValueUnknown
		return result
	}
	result.Number = value
	result.Status = ValueValid
	return result
}

// DecodeHex reads at most 32-bit unsigned hexadecimal values. It accepts
// optional &H and a closing &, exposing the consumed prefix for syntax lint.
func DecodeHex(raw string) DecodedArgument {
	result := DecodedArgument{Raw: raw, Kind: spec.HexValue, Status: ValueInvalid}
	pos := skipNumericSpace(raw, false)
	if hasHexPrefix(raw[pos:]) {
		pos += 2
	}
	signed := pos < len(raw) && (raw[pos] == '+' || raw[pos] == '-')
	if signed {
		pos++
	}
	start := pos
	for pos < len(raw) && hexDigit(raw[pos]) {
		pos++
	}
	if pos == start {
		return result
	}
	endDigits := pos
	if pos < len(raw) && raw[pos] == '&' {
		pos++
	}
	result.Consumed = pos
	if signed {
		result.Status = ValueUnknown
		return result
	}
	if endDigits-start > 8 {
		result.Status = ValueUnknown
		return result
	}
	value, err := strconv.ParseUint(raw[start:endDigits], 16, 32)
	if err != nil {
		result.Status = ValueUnknown
		return result
	}
	result.Hex = uint32(value)
	result.Status = ValueValid
	return result
}

// Exact decoders are used for proofs of equivalent render state. Prefix
// success alone must never justify a SafeFix.
func DecodeExactInteger(raw string) DecodedArgument {
	raw = strings.TrimSpace(raw)
	result := DecodeInteger(raw)
	if result.Status == ValueValid && result.Consumed != len(raw) {
		result.Status = ValueInvalid
	}
	return result
}

func DecodeExactNumber(raw string) DecodedArgument {
	raw = strings.TrimSpace(raw)
	result := DecodeNumber(raw)
	if result.Status == ValueValid && result.Consumed != len(raw) {
		result.Status = ValueInvalid
	}
	return result
}

func DecodeExactHex(raw string) DecodedArgument {
	raw = strings.TrimSpace(raw)
	result := DecodeHex(raw)
	if result.Status == ValueValid && result.Consumed != len(raw) {
		result.Status = ValueInvalid
	}
	return result
}

// DecodeRendererInteger represents the prefix-consuming 32-bit integer used
// in drawing-mode and style-reset reasoning. A missing number is renderer
// zero, but overflow and non-ASCII leading whitespace remain unknown.
func DecodeRendererInteger(raw string) DecodedArgument {
	result := DecodedArgument{Raw: raw, Kind: spec.IntegerValue, Status: ValueUnknown}
	start := skipNumericSpace(raw, true)
	if start < len(raw) && raw[start] >= 0x80 {
		return result
	}
	end, ok := integerEnd(raw, true)
	if !ok {
		result.Status = ValueValid
		result.Consumed = start
		return result
	}
	result.Consumed = end
	value, err := strconv.ParseInt(raw[start:end], 10, 32)
	if err != nil {
		return result
	}
	result.Integer = value
	result.Status = ValueValid
	return result
}

func integerEnd(raw string, renderer bool) (int, bool) {
	pos := skipNumericSpace(raw, renderer)
	if pos < len(raw) && (raw[pos] == '+' || raw[pos] == '-') {
		pos++
	}
	first := pos
	for pos < len(raw) && decimalDigit(raw[pos]) {
		pos++
	}
	return pos, pos != first
}

func skipNumericSpace(raw string, renderer bool) int {
	pos := 0
	for pos < len(raw) {
		switch raw[pos] {
		case ' ', '\t', '\n', '\r', '\f':
			pos++
		case '\v':
			if !renderer {
				return pos
			}
			pos++
		default:
			return pos
		}
	}
	return pos
}

func decimalDigit(c byte) bool { return c >= '0' && c <= '9' }
func hexDigit(c byte) bool {
	return decimalDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
func hasHexPrefix(s string) bool {
	return len(s) >= 2 && s[0] == '&' && (s[1] == 'H' || s[1] == 'h')
}
