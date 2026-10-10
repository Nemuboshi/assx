package ass

import (
	"testing"

	"assx/internal/ass/spec"
)

func TestSharedScalarDecoders(t *testing.T) {
	tests := []struct {
		name     string
		decode   func(string) DecodedArgument
		raw      string
		status   ValueStatus
		consumed int
	}{
		{"integer prefix", DecodeInteger, " \t-12junk", ValueValid, 5},
		{"integer overflow", DecodeInteger, "99999999999999999999999", ValueUnknown, 23},
		{"integer invalid", DecodeInteger, "junk", ValueInvalid, 0},
		{"decimal with exponent", DecodeNumber, "1.2e-3junk", ValueValid, 6},
		{"incomplete exponent", DecodeNumber, "1e+", ValueValid, 1},
		{"fraction", DecodeNumber, ".5", ValueValid, 2},
		{"number overflow", DecodeNumber, "1e999", ValueUnknown, 5},
		{"hex with prefix and terminator", DecodeHex, "&H80FF&junk", ValueValid, 7},
		{"hex signed unknown", DecodeHex, "+F", ValueUnknown, 2},
		{"hex missing digits", DecodeHex, "&H&", ValueInvalid, 0},
		{"hex overflow", DecodeHex, "&H123456789", ValueUnknown, 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.decode(tt.raw)
			if got.Status != tt.status || got.Consumed != tt.consumed {
				t.Fatalf("%q => (%v, %d), want (%v, %d)", tt.raw, got.Status, got.Consumed, tt.status, tt.consumed)
			}
		})
	}
}

func TestExactDecoderRejectsPrefixOnly(t *testing.T) {
	if got := DecodeExactNumber("1.5junk"); got.Status != ValueInvalid {
		t.Fatalf("number prefix was treated as exact: %#v", got)
	}
	if got := DecodeExactInteger("1e2"); got.Status != ValueInvalid {
		t.Fatalf("integer prefix was treated as exact: %#v", got)
	}
	if got := DecodeExactHex("&HFFFFFF&junk"); got.Status != ValueInvalid {
		t.Fatalf("hex prefix was treated as exact: %#v", got)
	}
	if value := DecodeExactNumber("  -0.0  "); value.Status != ValueValid {
		t.Fatalf("trimmed exact number rejected: %#v", value)
	}
	if normalized, ok := DecodeExactNumber(".5").Normalized(); !ok || normalized != "0.5" {
		t.Fatalf("normalized number = %q, %v", normalized, ok)
	}
}

func TestTagIRPreservesASTAndExposesSpec(t *testing.T) {
	source := "{\\fs20junk\\t(\\clip(1.5,0,30,40))}Text"
	tree := ParseDialogueText(source)
	if len(tree.Nodes) < 1 || tree.Nodes[0].Block == nil {
		t.Fatalf("missing parsed block: %#v", tree.Nodes)
	}
	tag := tree.Nodes[0].Block.Items[0].Tag
	if tag == nil || source[tag.Start:tag.End] != tag.Raw {
		t.Fatalf("source spans not lossless: %#v", tag)
	}
	ir := DecodeTag(*tag)
	if !ir.Known || ir.Spec.Value != spec.NumberValue || len(ir.Spec.Slots) != 1 || ir.Spec.Slots[0] != "fontsize" {
		t.Fatalf("spec metadata lost: %#v", ir)
	}
	value := ir.Argument(0)
	if value.Status != ValueValid || value.Number != 20 || value.Consumed != 2 {
		t.Fatalf("tag argument decoding = %#v", value)
	}
	if _, ok := value.Normalized(); ok {
		t.Fatal("prefix-only input must not be normalized")
	}
	if source[tag.Start:tag.End] != tag.Raw || tag.Raw != "\\fs20junk" {
		t.Fatal("decoding changed raw AST representation")
	}
}

func TestTagIRRendererAmbiguityAndStructuredArgs(t *testing.T) {
	tests := []struct {
		input  string
		status ValueStatus
	}{
		{`{\1c&HFFFFFF&}`, ValueValid},
		{`{\1c(&HFFFFFF&)}`, ValueAmbiguous},
		{`{\1c&hFFFFFF&}`, ValueAmbiguous},
		{`{\blur101}`, ValueAmbiguous},
		{`{\a4}`, ValueAmbiguous},
		{`{\clip(1.5,0,30,40)}`, ValueAmbiguous},
		{`{\clip(1.4,0,30,40)}`, ValueValid},
		{`{\pos(10,20)}`, ValueValid},
	}
	for _, tt := range tests {
		tree := ParseDialogueText(tt.input)
		tag := tree.Nodes[0].Block.Items[0].Tag
		ir := DecodeTag(*tag)
		got := ir.Argument(0)
		if got.Status != tt.status {
			t.Errorf("%s: status %v, want %v", tt.input, got.Status, tt.status)
		}
	}
}

func TestStructuredArgumentStatus(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		status ValueStatus
	}{
		{"pos", []string{"10", "20"}, ValueValid},
		{"pos", []string{"10"}, ValueInvalid},
		{"move", []string{"1", "2", "3", "4", "5", "6"}, ValueValid},
		{"move", []string{"1", "2", "3"}, ValueInvalid},
		{"pos", []string{"1junk", "2"}, ValueUnknown},
		{"clip", []string{"m 0 0 l 1 1"}, ValueUnknown},
		{"clip", []string{"1.5", "0", "10", "20"}, ValueAmbiguous},
	}
	for _, tc := range cases {
		ir := DecodeTag(Tag{Name: tc.name, Args: tc.args})
		if got := ir.ArgumentsStatus(); got != tc.status {
			t.Errorf("%s(%v): status %v, want %v", tc.name, tc.args, got, tc.status)
		}
	}
}

var decodedArgumentBenchSink DecodedArgument

func BenchmarkTypedTagDecode(b *testing.B) {
	tags := []Tag{
		{Name: "fs", Args: []string{"42.5"}},
		{Name: "1c", Args: []string{"&HFFFFFF&"}},
		{Name: "move", Args: []string{"0", "0", "100", "100", "500", "1000"}},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		ir := DecodeTag(tags[i%len(tags)])
		decodedArgumentBenchSink = ir.Argument(0)
	}
}

func TestRendererIntegerDefaultsAndOverflow(t *testing.T) {
	for _, test := range []struct {
		raw    string
		status ValueStatus
		value  int64
	}{
		{"", ValueValid, 0},
		{"  +2junk", ValueValid, 2},
		{"no number", ValueValid, 0},
		{"2147483648", ValueUnknown, 0},
		{"\u30001", ValueUnknown, 0},
	} {
		got := DecodeRendererInteger(test.raw)
		if got.Status != test.status || got.Integer != test.value {
			t.Errorf("%q: %#v", test.raw, got)
		}
	}
}
