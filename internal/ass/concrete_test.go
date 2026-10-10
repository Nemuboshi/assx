package ass

import (
	"strings"
	"testing"
)

func assertConcreteCoverage(t *testing.T, source string) ConcreteDialogue {
	t.Helper()
	tree := ParseConcreteDialogue(source)
	cursor := 0
	for _, node := range tree.Nodes {
		if node.Span.Start != cursor || node.Span.End <= cursor ||
			source[node.Span.Start:node.Span.End] != node.Raw {
			t.Fatalf("node coverage failed at %d: %#v", cursor, node)
		}
		cursor = node.Span.End
		if node.Block == nil {
			continue
		}
		block := node.Block
		if source[block.ContentSpan.Start:block.ContentSpan.End] != block.Content {
			t.Fatalf("content span mismatch: %#v", block)
		}
		position := block.ContentSpan.Start
		for _, item := range block.Items {
			if item.Span.Start != position || item.Span.End <= position ||
				source[item.Span.Start:item.Span.End] != item.Raw {
				t.Fatalf("item coverage failed at %d: %#v", position, item)
			}
			if expr := item.Expression; expr != nil {
				if expr.Span != item.Span || expr.Raw != item.Raw ||
					source[expr.SlashSpan.Start:expr.SlashSpan.End] != strings.Repeat("\\", expr.SlashSpan.End-expr.SlashSpan.Start) ||
					source[expr.HeadSpan.Start:expr.HeadSpan.End] != expr.Head {
					t.Fatalf("expression range mismatch: %#v", expr)
				}
				if expr.Parenthesized {
					if source[expr.Open] != '(' || expr.ContentSpan.Start != expr.Open+1 {
						t.Fatalf("opening parenthesis mismatch: %#v", expr)
					}
					if expr.Close >= 0 && (source[expr.Close] != ')' || expr.ContentSpan.End != expr.Close) {
						t.Fatalf("closing parenthesis mismatch: %#v", expr)
					}
					for _, comma := range expr.Commas {
						if comma < expr.ContentSpan.Start || comma >= expr.ContentSpan.End || source[comma] != ',' {
							t.Fatalf("comma outside arguments: %#v", expr)
						}
					}
				}
			}
			position = item.Span.End
		}
		if position != block.ContentSpan.End {
			t.Fatalf("block coverage ends at %d, want %d", position, block.ContentSpan.End)
		}
	}
	if cursor != len(source) {
		t.Fatalf("dialogue coverage ends at %d, want %d", cursor, len(source))
	}
	return tree
}

func TestConcreteDialoguePreservesRendererAmbiguity(t *testing.T) {
	source := "あ{\\fsvp6\\frs\\blend(add)\\pos(1,2,3)\\\\fs20\\future(x,(a,b),c)}終"
	tree := assertConcreteCoverage(t, source)
	if len(tree.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(tree.Nodes))
	}
	items := tree.Nodes[1].Block.Items
	if len(items) != 6 {
		t.Fatalf("items = %d, want 6: %#v", len(items), items)
	}
	for i, want := range []string{"fsvp6", "frs", "blend", "pos", "fs20", "future"} {
		expr := items[i].Expression
		if expr == nil || expr.Head != want {
			t.Fatalf("head at %d = %#v, want %s", i, expr, want)
		}
	}
	// No early commitment to either the fsvp or fs interpretation.
	if !strings.HasPrefix(items[0].Expression.Head, "fs") ||
		!strings.HasPrefix(items[0].Expression.Head, "fsvp") {
		t.Fatal("lost competing renderer dispatch candidates")
	}
	if len(items[4].Expression.Raw) != len("\\\\fs20") ||
		items[4].Expression.SlashSpan.End-items[4].Expression.SlashSpan.Start != 2 {
		t.Fatalf("slash run lost: %#v", items[4].Expression)
	}
	if got := items[5].Expression.Commas; len(got) != 2 {
		t.Fatalf("top-level comma positions = %v, want 2", got)
	}
}

func TestConcreteDialogueRetainsIncompleteAndUnknownSyntax(t *testing.T) {
	for _, source := range []string{
		"", "ordinary text", "}leading", "x{unterminated \\pos(1,2", "x{}y",
		"{\\}", "{\\\\}", "{\\pos(1,2}", "{\\t(0,10,\\clip(1,2)\\bord4}",
		"{\\mystery \\ future(foo,bar) junk \\}", "{\\t(1,2,\\clip(0,0,1,1))}",
		"{\\fs20} x \\text {comment without tag} y", "{\r\n\\fs3\t}",
	} {
		t.Run(source, func(t *testing.T) {
			assertConcreteCoverage(t, source)
		})
	}
	tree := ParseConcreteDialogue("one{unfinished")
	if len(tree.Nodes) != 2 || tree.Nodes[1].Kind != ConcreteUnclosedBrace {
		t.Fatalf("unterminated brace was not preserved: %#v", tree.Nodes)
	}
	expr := ParseConcreteDialogue("{\\pos(1,2}").Nodes[0].Block.Items[0].Expression
	if expr == nil || expr.Closed() || expr.ContentSpan.End != len("{\\pos(1,2") {
		t.Fatalf("unterminated expression was normalized: %#v", expr)
	}
}

func TestConcreteDialogueDeepParenthesesWithoutRecursiveFrames(t *testing.T) {
	const depth = 20000
	source := "{\\t(" + strings.Repeat("(", depth) + "x" + strings.Repeat(")", depth) + ")}"
	tree := assertConcreteCoverage(t, source)
	expr := tree.Nodes[0].Block.Items[0].Expression
	if expr == nil || !expr.Closed() || expr.Close != len(source)-2 {
		t.Fatalf("deep expression = %#v", expr)
	}
}

func FuzzConcreteDialogueSourceCoverage(f *testing.F) {
	for _, source := range []string{
		"", "abc", "é日本語{\\fsvp6\\pos(1,2,3)}", "{\\t(1,\\clip(1,2,3))}",
		"{\\\\fs20\\unknown(x,(y,z))}", "{\\t(", "abc{", "{\\}\r\n",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		// Cap adversarial allocations while exercising arbitrary byte strings.
		if len(source) > 32768 {
			t.Skip()
		}
		assertConcreteCoverage(t, source)
	})
}
