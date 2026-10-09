package ass

import (
	"reflect"
	"strings"
	"testing"
)

func TestNestedParenthesesInTransformAndEmptyClip(t *testing.T) {
	text := `prefix{\t(18,3438,\clip())\alpha&H00&}suffix`
	tree := ParseDialogueText(text)
	tags := tree.Tokens()
	var transform, clip *Tag
	for _, token := range tags {
		if token.Tag == nil {
			continue
		}
		if token.Tag.Name == "t" {
			transform = token.Tag
		}
		if token.Tag.Name == "clip" {
			clip = token.Tag
		}
	}
	if transform == nil || len(transform.Children) != 1 || transform.Children[0].Name != "clip" {
		t.Fatalf("transform = %#v", transform)
	}
	if clip == nil || len(clip.Args) != 0 || text[clip.End:strings.Index(text, `\alpha`)] != ")" {
		t.Fatalf("clip = %#v; parser did not consume only its own parentheses", clip)
	}
}

func TestDeeplyNestedTransformsUseExplicitFrames(t *testing.T) {
	const depth = 10000
	text := "{" + strings.Repeat(`\t(0,1,`, depth) + `\fscx100` + strings.Repeat(")", depth) + "}"
	tokens := ParseDialogueText(text).Tokens()
	if len(tokens) != depth+1 {
		t.Fatalf("got %d flattened tags, want %d", len(tokens), depth+1)
	}
}

func TestWalkTokensMatchesTokensAndStopsEarly(t *testing.T) {
	tree := ParseDialogueText(`a{\fs20\t(0,500,\blur1)}b{\\bord2}c`)
	var walked []Token
	tree.WalkTokens(func(view TokenView) bool {
		if view.HasTag {
			tag := view.Tag
			walked = append(walked, Token{Tag: &tag})
		} else {
			walked = append(walked, Token{Text: view.Text, Start: view.Start})
		}
		return true
	})
	if !reflect.DeepEqual(walked, tree.Tokens()) {
		t.Fatalf("visitor tokens = %#v, materialized tokens = %#v", walked, tree.Tokens())
	}
	visited := 0
	tree.WalkTokens(func(TokenView) bool {
		visited++
		return false
	})
	if visited != 1 {
		t.Fatalf("early-stop visitor called %d times, want 1", visited)
	}
}

func TestDialogueTokensMarkTransitionTagsAndSourceSpans(t *testing.T) {
	text := `pre{\blur6\t(0,650,0.1,\blur0.6)}post`
	tree := ParseDialogueText(text)
	tokens := tree.Tokens()

	var tags []Tag
	for _, token := range tokens {
		if token.Tag != nil {
			tags = append(tags, *token.Tag)
		}
	}
	if len(tags) != 3 {
		t.Fatalf("got %d tags, want 3: %#v", len(tags), tags)
	}
	if tags[0].Name != "blur" || tags[0].InTransition {
		t.Fatalf("outer tag = %#v", tags[0])
	}
	if tags[2].Name != "blur" || !tags[2].InTransition {
		t.Fatalf("nested tag = %#v", tags[2])
	}
	if got := text[tags[2].Start:tags[2].End]; got != `\blur0.6` {
		t.Fatalf("nested source span = %q", got)
	}
	if len(tags[1].Children) != 1 || tags[1].Children[0].Name != "blur" {
		t.Fatalf("transform children = %#v", tags[1].Children)
	}
}

func TestDialogueTokensPreserveLegacyRepeatedSlashWalk(t *testing.T) {
	tokens := ParseDialogueText(`{\\blur2\mystery3} outside \\text`).Tokens()
	if len(tokens) != 4 {
		t.Fatalf("got %d tokens: %#v", len(tokens), tokens)
	}
	if tokens[0].Tag == nil || tokens[0].Tag.RepeatedSlashes != 1 {
		t.Fatalf("first token = %#v, want repeated slash marker", tokens[0])
	}
	if tokens[1].Tag == nil || tokens[1].Tag.Name != "blur" || tokens[1].Tag.RepeatedSlashes != 0 {
		t.Fatalf("second token = %#v, want blur tag", tokens[1])
	}
	if tokens[2].Tag == nil || tokens[2].Tag.Name != "mystery3" {
		t.Fatalf("third token = %#v, want unknown tag", tokens[2])
	}
	if tokens[3].Tag != nil || tokens[3].Text != " outside \\\\text" {
		t.Fatalf("outside text token = %#v", tokens[3])
	}

	triple := ParseDialogueText(`{\\\blur1}`).Tokens()
	if len(triple) != 2 || triple[0].Tag == nil || triple[0].Tag.RepeatedSlashes != 2 || triple[1].Tag == nil || triple[1].Tag.Name != "blur" {
		t.Fatalf("three-slash tokenization = %#v", triple)
	}
}

func TestDialogueASTPreservesBlockRawItems(t *testing.T) {
	text := `a{ junk \fs20 more \bord2 }b{}c`
	tree := ParseDialogueText(text)
	if len(tree.Nodes) != 5 {
		t.Fatalf("nodes = %#v", tree.Nodes)
	}

	block := tree.Nodes[1].Block
	if block == nil {
		t.Fatal("missing first block")
	}
	if len(block.Tags()) != 2 || block.Tags()[0].Name != "fs" || block.Tags()[1].Name != "bord" {
		t.Fatalf("block tags = %#v", block.Tags())
	}

	var raw string
	for _, item := range block.Items {
		if item.Tag == nil {
			raw += item.Raw
		}
	}
	if raw != " junk " {
		t.Fatalf("raw block content = %q", raw)
	}
	tags := block.Tags()
	if tags[0].Raw != `\fs20 more ` || len(tags[0].Args) != 1 || tags[0].Args[0] != "20 more" {
		t.Fatalf("first tag did not preserve raw source: %#v", tags[0])
	}

	empty := tree.Nodes[3].Block
	if empty == nil || empty.HasTags() || len(empty.Items) != 0 {
		t.Fatalf("empty block = %#v", empty)
	}
}

func TestParsedDocumentCachesDialogueSyntax(t *testing.T) {
	doc := Parse("[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0,1,Default,{\\fs20}x\n")
	if len(doc.Dialogues) != 1 {
		t.Fatalf("dialogues = %#v", doc.Dialogues)
	}
	dialogue := doc.Dialogues[0]
	if dialogue.Syntax.Source != dialogue.Text || len(dialogue.Syntax.Nodes) == 0 {
		t.Fatalf("syntax was not cached: %#v", dialogue.Syntax)
	}
	if got := dialogue.ParsedText(); got.Source != dialogue.Text {
		t.Fatalf("ParsedText source = %q", got.Source)
	}
}
