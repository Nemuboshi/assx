package lsp

import (
	"testing"

	"assx/internal/sourcepos"
)

func TestDocumentsRefreshParsedSnapshotAndIndex(t *testing.T) {
	var store Documents
	uri := "file:///test.ass"
	if err := store.Open(uri, 1, "[Events]\nFormat: Start, End, Style, Text\nDialogue: 0:00:00.00,0:00:01.00,Default,old"); err != nil {
		t.Fatal(err)
	}
	if err := store.Change(uri, 2, []Change{{Text: "[Events]\nFormat: Start, End, Style, Text\nDialogue: 0:00:00.00,0:00:01.00,Default,new😀"}}); err != nil {
		t.Fatal(err)
	}
	current := store.items[uri]
	if len(current.parsed.Dialogues) != 1 || current.parsed.Dialogues[0].Text != "new😀" {
		t.Fatalf("parsed snapshot does not match version 2: %+v", current.parsed.Dialogues)
	}
	position, err := current.index.Position(len(current.text))
	if err != nil || position.Line != 2 || position.Character != 45 {
		t.Fatalf("position = %+v, %v", position, err)
	}
}

func TestSequentialUnicodeChangesAreAtomic(t *testing.T) {
	var store Documents
	uri := "file:///C:/sub%20titles/test.ass"
	if err := store.Open(uri, 1, "中😀x"); err != nil {
		t.Fatal(err)
	}
	changes := []Change{
		{Range: &sourcepos.Range{Start: sourcepos.Position{Line: 0, Character: 1}, End: sourcepos.Position{Line: 0, Character: 3}}, Text: "日"},
		{Range: &sourcepos.Range{Start: sourcepos.Position{Line: 0, Character: 2}, End: sourcepos.Position{Line: 0, Character: 2}}, Text: "\n"},
	}
	if err := store.Change(uri, 2, changes); err != nil {
		t.Fatal(err)
	}
	if got, version, ok := store.Snapshot(uri); !ok || got != "中日\nx" || version != 2 {
		t.Fatalf("snapshot = %q, %d, %t", got, version, ok)
	}
	if err := store.Change(uri, 3, []Change{{Range: &sourcepos.Range{Start: sourcepos.Position{Line: 0, Character: 1}, End: sourcepos.Position{Line: 0, Character: 2}}, Text: "bad"}, {Range: &sourcepos.Range{Start: sourcepos.Position{Line: 9, Character: 0}, End: sourcepos.Position{Line: 9, Character: 0}}, Text: "x"}}); err == nil {
		t.Fatal("accepted partially invalid edit batch")
	}
	if got, version, _ := store.Snapshot(uri); got != "中日\nx" || version != 2 {
		t.Fatalf("failed change altered snapshot: %q, %d", got, version)
	}
	if err := store.Change(uri, 2, []Change{{Text: "old"}}); err == nil {
		t.Fatal("accepted old version")
	}
	store.Close(uri)
	if _, _, ok := store.Snapshot(uri); ok {
		t.Fatal("closed document retained")
	}
}

func TestDocumentsRejectInvalidURIAndSurrogateEdit(t *testing.T) {
	var store Documents
	for _, uri := range []string{"", "plain.ass", "file:///bad%ZZ"} {
		if err := store.Open(uri, 1, "x"); err == nil {
			t.Errorf("accepted URI %q", uri)
		}
	}
	uri := "untitled:ASS-1"
	if err := store.Open(uri, 1, "😀"); err != nil {
		t.Fatal(err)
	}
	if err := store.Change(uri, 2, []Change{{Range: &sourcepos.Range{Start: sourcepos.Position{Line: 0, Character: 1}, End: sourcepos.Position{Line: 0, Character: 2}}, Text: "x"}}); err == nil {
		t.Fatal("accepted edit inside surrogate pair")
	}
	if got, version, _ := store.Snapshot(uri); got != "😀" || version != 1 {
		t.Fatalf("rejected edit changed document: %q, %d", got, version)
	}
}
