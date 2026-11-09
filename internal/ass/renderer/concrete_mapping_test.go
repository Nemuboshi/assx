package renderer

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"assx/internal/ass"
)

func TestDocumentDialogueConcreteSourceEquality(t *testing.T) {
	doc := Parse("[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0,1,Default,{\\fs20}x\n")
	if len(doc.Dialogues) != 1 {
		t.Fatalf("dialogues=%#v", doc.Dialogues)
	}
	d := doc.Dialogues[0]
	tree := ass.ParseConcreteDialogue(d.Text)
	if tree.Source != doc.Text[d.TextStart:d.TextStart+len(d.Text)] {
		t.Fatal("dialogue source differs")
	}
}

func TestConcreteDocumentEncodingFixturesAndDialogueCoordinateMapping(t *testing.T) {
	for _, name := range []string{
		"ordinary.ass", "renderer-collisions.ass", "utf16le.ass", "utf8bom.ass",
	} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "contracts", "cases", name))
			if err != nil {
				t.Fatal(err)
			}
			source, err := ass.DecodeSource(input)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(source.Encode(source.Text), input) {
				t.Fatal("BOM or original source encoding changed during round-trip")
			}
			concrete := ass.ParseConcreteDocument(source.Text)
			cursor := 0
			for _, line := range concrete.Lines {
				if line.Span.Start != cursor || line.Raw != source.Text[line.Span.Start:line.Span.End] {
					t.Fatalf("line span invalid: %#v", line)
				}
				cursor = line.Span.End
			}
			if cursor != len(source.Text) {
				t.Fatalf("ended at %d, expected %d", cursor, len(source.Text))
			}
			for _, dialogue := range Parse(source.Text).Dialogues {
				tree := ass.ParseConcreteDialogue(dialogue.Text)
				for _, node := range tree.Nodes {
					absolute := node.Span.AtDocumentOffset(dialogue.TextStart)
					if source.Text[absolute.Start:absolute.End] != node.Raw {
						t.Fatalf("dialogue span mapping failed: %#v", absolute)
					}
				}
			}
		})
	}
}
