package lint

import (
	"testing"

	"assx/internal/ass"
)

func TestDialogueEditIndexLookup(t *testing.T) {
	doc := ass.Document{Text: "Dialogue: \\pos(1,2)", Dialogues: []ass.Dialogue{{TextStart: 10, Text: `{\\pos(1,2)}`}}}
	index := NewDialogueEditIndex(doc)

	tree, ok := index.DialogueTree(10)
	if !ok {
		t.Fatal("expected indexed dialogue")
	}
	if !tree.HasCandidates() {
		t.Fatal("expected indexed concrete tree")
	}
}

func TestDialogueEditIndexMiss(t *testing.T) {
	index := NewDialogueEditIndex(ass.Document{})
	if _, ok := index.DialogueTree(1); ok {
		t.Fatal("unexpected index hit")
	}
}
