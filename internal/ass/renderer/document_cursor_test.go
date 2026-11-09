package renderer

import (
	"testing"

	"assx/internal/ass"
)

func TestDocumentCursorTracksSectionAndFormats(t *testing.T) {
	tree := ass.ParseConcreteDocument("[Events]\nFormat: Layer, Text\n[V4+ Styles]\nName: x\n")
	var cursor documentCursor
	for _, line := range tree.Lines {
		cursor.enter(line)
	}
	if cursor.Section() != "v4+ styles" {
		t.Fatalf("section = %q", cursor.Section())
	}
	cursor.setEventFormat("Layer, actor", true)
	if cursor.eventFormat[1] != "name" {
		t.Fatalf("event format = %v", cursor.eventFormat)
	}
	cursor.setStyleFormat("Name, Actor", true)
	if cursor.styleFormat[1] != "name" {
		t.Fatalf("style format = %v", cursor.styleFormat)
	}
	cursor.resetStyleFormat()
	if cursor.styleFormatOrDefault(func(string) []string { return []string{"fallback"} })[0] != "fallback" {
		t.Fatalf("style default = %v", cursor.styleFormat)
	}
}

func TestRecordKeyKeepsExactAndLowerForms(t *testing.T) {
	tree := ass.ParseConcreteDocument("Format: Layer\n  Format : Text\n")
	key, exact, ok := recordKey(tree.Lines[0])
	if !ok || key != "format" || exact != "Format" {
		t.Fatalf("record key = %q, %q, %t", key, exact, ok)
	}
	key, exact, ok = recordKey(tree.Lines[1])
	if !ok || key != "format" || exact != "Format " {
		t.Fatalf("indented record key = %q, %q, %t", key, exact, ok)
	}
}
