package ass

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
		tag := ParseDialogueText(test.text).Nodes[0].Block.Items[0].Tag
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
		tree := ParseDialogueText(test.text)
		tag := tree.Nodes[0].Block.Items[0].Tag
		value, known := tag.IntegerArgument()
		if known != test.known || (known && value != test.want) {
			t.Fatalf("%s: (%d, %v), want (%d, %v)", test.text, value, known, test.want, test.known)
		}
	}
}

func TestBareDrawingTagEndsDrawingMode(t *testing.T) {
	for _, reset := range []string{`{\p}`, `{\p()}`, `{\p0}`, `{\p-1}`} {
		drawings := ParseDialogueText(`{\p1}m 0 0 l 10 10` + reset + "visible text").Drawings()
		if len(drawings) != 1 || len(drawings[0].Issues) != 0 || len(drawings[0].Commands) != 2 {
			t.Fatalf("reset %s leaked visible text into drawing: %#v", reset, drawings)
		}
	}
}
