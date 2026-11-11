package renderer

import (
	"math"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

func TestDecodeIntegerBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		status   ValueStatus
		integer  int64
		consumed int
	}{
		{"max int64", "9223372036854775807", ValueValid, math.MaxInt64, 19},
		{"max int64 overflow", "9223372036854775808", ValueUnknown, 0, 19},
		{"min int64", "-9223372036854775808", ValueValid, math.MinInt64, 20},
		{"min int64 overflow", "-9223372036854775809", ValueUnknown, 0, 20},
		{"long overflow", "999999999999999999999999999999", ValueUnknown, 0, 30},
		{"sign and prefix", "+42junk", ValueValid, 42, 3},
		{"fraction prefix", "42.5", ValueValid, 42, 2},
		{"leading ascii space", " \t\r\n\f7", ValueValid, 7, 6},
		{"trailing space", "7  junk", ValueValid, 7, 1},
		{"sign only", "+", ValueInvalid, 0, 0},
		{"double sign", "+-1", ValueInvalid, 0, 0},
		{"unicode space", "\u00a07", ValueInvalid, 0, 0},
		{"ideographic space", "\u30007", ValueInvalid, 0, 0},
		{"vertical tab", "\v7", ValueInvalid, 0, 0},
		{"empty", "", ValueInvalid, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeInteger(tc.raw)
			if got.Status != tc.status || got.Integer != tc.integer || got.Consumed != tc.consumed {
				t.Fatalf("%q => %#v, want status %v value %d consumed %d", tc.raw, got, tc.status, tc.integer, tc.consumed)
			}
		})
	}
}

func TestDecodeNumberBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		status   ValueStatus
		number   float64
		consumed int
	}{
		{"max float64", "1.7976931348623157e308", ValueValid, math.MaxFloat64, 22},
		{"float64 overflow", "1.7976931348623159e308", ValueUnknown, 0, 22},
		{"exponent overflow", "1e999", ValueUnknown, 0, 5},
		{"huge exponent", "1e999999999999999999999", ValueUnknown, 0, 23},
		{"underflow", "1e-999", ValueValid, 0, 6},
		{"incomplete exponent", "1e+", ValueValid, 1, 1},
		{"bare exponent", "1e", ValueValid, 1, 1},
		{"complete exponent", "1e+1x", ValueValid, 10, 4},
		{"fraction only", ".5", ValueValid, 0.5, 2},
		{"signed fraction", "+.5", ValueValid, 0.5, 3},
		{"trailing dot", "1..2", ValueValid, 1, 2},
		{"no digits", ".", ValueInvalid, 0, 0},
		{"exponent only", "e5", ValueInvalid, 0, 0},
		{"infinity text", "inf", ValueInvalid, 0, 0},
		{"nan text", "nan", ValueInvalid, 0, 0},
		{"hex prefix reads zero", "0x10", ValueValid, 0, 1},
		{"unicode space", "\u00a01.5", ValueInvalid, 0, 0},
		{"vertical tab", "\v1.5", ValueInvalid, 0, 0},
		{"negative zero", "-0.0", ValueValid, 0, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeNumber(tc.raw)
			if got.Status != tc.status || got.Number != tc.number || got.Consumed != tc.consumed {
				t.Fatalf("%q => %#v, want status %v value %v consumed %d", tc.raw, got, tc.status, tc.number, tc.consumed)
			}
		})
	}
}

func TestDecodeHexBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		status   ValueStatus
		hex      uint32
		consumed int
	}{
		{"max uint32", "&HFFFFFFFF", ValueValid, math.MaxUint32, 10},
		{"uint32 overflow", "&H100000000", ValueUnknown, 0, 11},
		{"long overflow", "&HFFFFFFFFFFFFFFFF", ValueUnknown, 0, 18},
		{"bare digits", "FFFFFFFF", ValueValid, math.MaxUint32, 8},
		{"lowercase prefix", "&hff", ValueValid, 0xff, 4},
		{"terminator", "&HFF&", ValueValid, 0xff, 5},
		{"terminator then junk", "&HFF&junk", ValueValid, 0xff, 5},
		{"prefix only", "&H", ValueInvalid, 0, 0},
		{"empty body", "&H&", ValueInvalid, 0, 0},
		{"signed", "&H-1", ValueUnknown, 0, 4},
		{"signed no prefix", "+F", ValueUnknown, 0, 2},
		{"leading ascii space", "  &H10  ", ValueValid, 0x10, 6},
		{"unicode space", "\u00a0&H10", ValueInvalid, 0, 0},
		{"no digit", "&HG", ValueInvalid, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeHex(tc.raw)
			if got.Status != tc.status || got.Hex != tc.hex || got.Consumed != tc.consumed {
				t.Fatalf("%q => %#v, want status %v value %#x consumed %d", tc.raw, got, tc.status, tc.hex, tc.consumed)
			}
		})
	}
}

// TestExactDecodersRejectUnicodeSpace keeps Unicode whitespace from turning a
// renderer-dependent argument into an exact value. Only the ASCII whitespace
// accepted by the prefix grammar may be trimmed.
func TestExactDecodersRejectUnicodeSpace(t *testing.T) {
	for _, raw := range []string{"\u00a042", "\u200342", "\u300042", "\v42"} {
		if got := DecodeExactInteger(raw); got.Status != ValueInvalid {
			t.Errorf("exact integer %q = %#v, want invalid", raw, got)
		}
		if got := DecodeExactNumber(raw); got.Status != ValueInvalid {
			t.Errorf("exact number %q = %#v, want invalid", raw, got)
		}
	}
	if got := DecodeExactHex("\u00a0&H10"); got.Status != ValueInvalid {
		t.Errorf("exact hex = %#v, want invalid", got)
	}
	if got := DecodeExactInteger(" \t42\r\n"); got.Status != ValueValid || got.Integer != 42 {
		t.Errorf("ascii padded exact integer = %#v", got)
	}
}

func TestDecodeRendererIntegerBoundaries(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		status   ValueStatus
		integer  int64
		consumed int
	}{
		{"empty is zero", "", ValueValid, 0, 0},
		{"space is zero", "  ", ValueValid, 0, 2},
		{"missing number", "junk", ValueValid, 0, 0},
		{"max int32", "2147483647", ValueValid, math.MaxInt32, 10},
		{"int32 overflow", "2147483648", ValueUnknown, 0, 10},
		{"min int32", "-2147483648", ValueValid, math.MinInt32, 11},
		{"min int32 overflow", "-2147483649", ValueUnknown, 0, 11},
		{"prefix junk", "42junk", ValueValid, 42, 2},
		{"vertical tab accepted", "\v42", ValueValid, 42, 3},
		{"unicode space rejected", "\u00a042", ValueUnknown, 0, 0},
		{"sign only", "+", ValueValid, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecodeRendererInteger(tc.raw)
			if got.Status != tc.status || got.Integer != tc.integer || got.Consumed != tc.consumed {
				t.Fatalf("%q => %#v, want status %v value %d consumed %d", tc.raw, got, tc.status, tc.integer, tc.consumed)
			}
		})
	}
}

func TestStructuredArgumentsWithIgnoredTails(t *testing.T) {
	cases := []struct {
		name   string
		tag    string
		args   []string
		status ValueStatus
	}{
		{"pos tail", "pos", []string{"10junk", "20"}, ValueUnknown},
		{"move tail", "move", []string{"0", "0", "100junk", "100"}, ValueUnknown},
		{"clip tail", "clip", []string{"1.5", "0", "30junk", "40"}, ValueUnknown},
		{"partial rectangle", "clip", []string{"1", "0", "30"}, ValueInvalid},
		{"vector clip", "clip", []string{"m 0 0 l 1 1"}, ValueUnknown},
		{"malformed vector", "clip", []string{"m 0 l"}, ValueUnknown},
		{"five argument move", "move", []string{"0", "0", "100", "100", "0"}, ValueInvalid},
		{"exact rectangle", "clip", []string{"1", "0", "30", "40"}, ValueValid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ir := DecodeTagWithSpec(Tag{Name: tc.tag, Args: tc.args}, spec.TagSpecs[tc.tag], true)
			if got := ir.ArgumentsStatus(); got != tc.status {
				t.Fatalf("%s(%v) status = %v, want %v", tc.tag, tc.args, got, tc.status)
			}
		})
	}
	ir := DecodeTagWithSpec(Tag{Name: "move", Args: []string{"0", "0", "100junk", "100"}}, spec.TagSpecs["move"], true)
	if got := ir.Argument(2); got.Consumed != 3 || got.Status != ValueValid {
		t.Fatalf("ignored tail consumed bytes changed: %#v", got)
	}
}

// TestTagIRKeepsUTF8ByteOffsets checks that decoding does not move the raw
// argument or its UTF-8 source span.
func TestTagIRKeepsUTF8ByteOffsets(t *testing.T) {
	source := "前{\\fs\u00a020junk}後"
	tree := ass.ParseConcreteDialogue(source)
	var expression *ass.ConcreteExpression
	for _, node := range tree.Nodes {
		if node.Block == nil || len(node.Block.Items) == 0 || node.Block.Items[0].Expression == nil {
			continue
		}
		expression = node.Block.Items[0].Expression
		break
	}
	if expression == nil {
		t.Fatal("missing resolved expression")
	}
	profile, _ := Standard(Libass)
	resolved := profile.Resolve(*expression, source)
	if source[resolved.Source.Start:resolved.Source.End] != resolved.Raw {
		t.Fatalf("tag span moved: %q", resolved.Raw)
	}
	if resolved.Source.Start != len("前{") {
		t.Fatalf("tag start = %d, want %d", resolved.Source.Start, len("前{"))
	}
	if len(resolved.Args) != 1 {
		t.Fatalf("arguments = %#v", resolved.Args)
	}
	arg := resolved.Args[0]
	if arg.Raw != "\u00a020junk" || source[arg.Span.Start:arg.Span.End] != arg.Raw {
		t.Fatalf("argument raw or span changed: %#v", arg)
	}
	tag := Tag{Name: resolved.Name, Raw: resolved.Raw, Start: resolved.Source.Start, End: resolved.Source.End, Paren: resolved.Form == Paren, Args: []string{arg.Raw}}
	if got := DecodeTagWithSpec(tag, spec.TagSpecs[tag.Name], true).Argument(0); got.Status != ValueInvalid {
		t.Fatalf("unicode prefixed argument became exact: %#v", got)
	}
}
