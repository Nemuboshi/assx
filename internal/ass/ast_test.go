package ass

import (
	"strings"
	"testing"
)

func TestNestedParenthesesInTransformAndEmptyClip(t *testing.T) {
	source := `prefix{\t(18,3438,\clip())\alpha&H00&}suffix`
	tree := ParseConcreteDialogue(source)
	var expressions []ConcreteExpression
	WalkConcreteExpressions(tree, func(e ConcreteExpression, nested bool) bool { expressions = append(expressions, e); return true })
	if len(expressions) != 3 || expressions[1].Head != "clip" || !expressions[1].Closed() || expressions[1].ContentSpan.Start != expressions[1].ContentSpan.End {
		t.Fatalf("expressions: %#v", expressions)
	}
	if source[expressions[1].Span.End:strings.Index(source, `\alpha`)] != ")" {
		t.Fatal("nested expression consumed its parent")
	}
}

func TestDeeplyNestedTransformsUseExplicitFrames(t *testing.T) {
	const depth = 10000
	source := "{" + strings.Repeat(`\t(0,1,`, depth) + `\fscx100` + strings.Repeat(")", depth) + "}"
	count := 0
	WalkConcreteExpressions(ParseConcreteDialogue(source), func(e ConcreteExpression, nested bool) bool { count++; return true })
	if count != depth+1 {
		t.Fatalf("expressions=%d", count)
	}
}

func TestConcreteWalkSourceOrderAndStopsEarly(t *testing.T) {
	source := `a{\fs20\t(0,500,\blur1)}b{\\bord2}c`
	tree := ParseConcreteDialogue(source)
	previous := -1
	WalkConcreteExpressions(tree, func(e ConcreteExpression, nested bool) bool {
		if e.Span.Start <= previous || source[e.Span.Start:e.Span.End] != e.Raw {
			t.Fatal("invalid order or raw span")
		}
		previous = e.Span.Start
		return true
	})
	count := 0
	WalkConcreteExpressions(tree, func(ConcreteExpression, bool) bool { count++; return false })
	if count != 1 {
		t.Fatalf("early stop visited %d", count)
	}
}

func TestConcreteNestedSourceSpans(t *testing.T) {
	source := `pre{\blur6\t(0,650,0.1,\blur0.6)}post`
	count := 0
	WalkConcreteExpressions(ParseConcreteDialogue(source), func(e ConcreteExpression, nested bool) bool {
		count++
		if count == 3 && (!nested || e.Raw != `\blur0.6` || source[e.Span.Start:e.Span.End] != e.Raw) {
			t.Fatal("lost nested span")
		}
		return true
	})
	if count != 3 {
		t.Fatalf("count=%d", count)
	}
}

func TestConcreteRepeatedSlashRunsStayLossless(t *testing.T) {
	for _, source := range []string{`{\\blur2\mystery3} outside \\text`, `{\\\blur1}`} {
		tree := ParseConcreteDialogue(source)
		first := tree.Nodes[0].Block.Items[0].Expression
		want := 2
		if strings.HasPrefix(source, `{\\\`) {
			want = 3
		}
		if first.SlashSpan.End-first.SlashSpan.Start != want {
			t.Fatal("lost slash run")
		}
		var raw strings.Builder
		for _, node := range tree.Nodes {
			raw.WriteString(node.Raw)
		}
		if raw.String() != source {
			t.Fatal("lost source bytes")
		}
	}
}

func TestConcretePreservesBlockRawItems(t *testing.T) {
	source := `a{ junk \fs20 more \bord2 }b{}c`
	tree := ParseConcreteDialogue(source)
	if len(tree.Nodes) != 5 {
		t.Fatalf("nodes=%#v", tree.Nodes)
	}
	block := tree.Nodes[1].Block
	if len(block.Items) != 3 || block.Items[0].Raw != " junk " || block.Items[1].Expression.Head != "fs20 more " {
		t.Fatalf("block=%#v", block)
	}
	var raw strings.Builder
	for _, item := range block.Items {
		raw.WriteString(item.Raw)
	}
	if raw.String() != block.Content {
		t.Fatal("lost raw items")
	}
	if empty := tree.Nodes[3].Block; empty == nil || len(empty.Items) != 0 {
		t.Fatalf("empty=%#v", empty)
	}
}

func TestDocumentDialogueConcreteSourceEquality(t *testing.T) {
	doc := Parse("[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0,1,Default,{\\fs20}x\n")
	if len(doc.Dialogues) != 1 {
		t.Fatalf("dialogues=%#v", doc.Dialogues)
	}
	d := doc.Dialogues[0]
	tree := ParseConcreteDialogue(d.Text)
	if tree.Source != doc.Text[d.TextStart:d.TextStart+len(d.Text)] {
		t.Fatal("dialogue source differs")
	}
}
