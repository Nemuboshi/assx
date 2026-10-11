package ass_test

import "testing"

func TestRawArgumentPreservesRendererResetSyntax(t *testing.T) {
	for _, test := range []struct {
		text, want string
		known      bool
	}{
		{`{\fn 0}`, " 0", true}, {`{\fn0 }`, "0 ", true},
		{`{\b( )}`, " ", true}, {`{\fn}`, "", true},
		{`{\fn( 0 )}`, " 0 ", true}, {`{\b1(0)}`, "", false},
		{`{\b(}`, "", false}, {"{\\i\u30001}", "\u30001", true},
	} {
		tag := resolvedTestTag(t, test.text)
		value, known := tag.RawArgument()
		if value != test.want || known != test.known {
			t.Fatalf("%s: (%q, %v), want (%q, %v)", test.text, value, known, test.want, test.known)
		}
	}
}

func TestIntegerArgumentRendererSyntax(t *testing.T) {
	for _, test := range []struct {
		text  string
		want  int32
		known bool
	}{
		{`{\p}`, 0, true}, {`{\p()}`, 0, true}, {`{\p1}`, 1, true},
		{`{\p-1}`, -1, true}, {`{\p( +2junk)}`, 2, true},
		{`{\p(no number)}`, 0, true}, {`{\p2147483648}`, 0, false},
		{`{\p(1,2)}`, 0, false},
	} {
		tag := resolvedTestTag(t, test.text)
		value, known := tag.IntegerArgument()
		if known != test.known || (known && value != test.want) {
			t.Fatalf("%s: (%d, %v), want (%d, %v)", test.text, value, known, test.want, test.known)
		}
	}
}
