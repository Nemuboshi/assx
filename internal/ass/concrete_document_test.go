package ass

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConcreteNestedComponentsPreserveOpaqueRendererCandidates(t *testing.T) {
	source := "前{\\t(0,100,\\clip(0,0,10,10)\\t(1,2,\\frs)\\pos(1,2,3))\\fsvp6}後"
	tree := assertConcreteCoverage(t, source)
	block := tree.Nodes[1].Block
	if len(block.Items) != 2 {
		t.Fatalf("outer items = %#v", block.Items)
	}
	outer := block.Items[0].Expression
	if outer == nil || outer.Head != "t" || !outer.Closed() {
		t.Fatalf("outer transform candidate = %#v", outer)
	}
	components := outer.Components(source)
	if len(components) != 3 || components[0].Raw != "0" || components[1].Raw != "100" {
		t.Fatalf("outer components = %#v", components)
	}
	nested := components[2].Items
	if len(nested) != 3 {
		t.Fatalf("nested candidate count = %d, want 3: %#v", len(nested), nested)
	}
	for i, want := range []string{"clip", "t", "pos"} {
		if nested[i].Expression == nil || nested[i].Expression.Head != want {
			t.Fatalf("nested candidate %d: %#v", i, nested[i])
		}
	}
	nestedTransform := nested[1].Expression.Components(source)
	if len(nestedTransform) != 3 || nestedTransform[2].Items[0].Expression.Head != "frs" {
		t.Fatalf("nested transform content lost: %#v", nestedTransform)
	}
	if block.Items[1].Expression.Head != "fsvp6" {
		t.Fatalf("extension head lost: %#v", block.Items[1].Expression)
	}
}

func TestConcreteNestedComponentsRetainMalformedArguments(t *testing.T) {
	for _, source := range []string{
		"{\\t(0,1,\\clip(1,2)}", "{\\t(,\\pos(1,2,3)}",
		"{\\future(foo,(a,b),bar)}", "{\\t(0,1,\\t(0,1,\\alpha&HFF&))}",
	} {
		t.Run(source, func(t *testing.T) {
			tree := assertConcreteCoverage(t, source)
			expr := tree.Nodes[0].Block.Items[0].Expression
			if expr == nil || !expr.Parenthesized {
				t.Fatalf("expected parenthesized syntax: %#v", expr)
			}
			components := expr.Components(source)
			if len(components) != len(expr.Commas)+1 {
				t.Fatalf("components = %d, commas = %d", len(components), len(expr.Commas))
			}
			for i, component := range components {
				if component.Raw != source[component.Span.Start:component.Span.End] {
					t.Fatalf("component %d not source-backed: %#v", i, component)
				}
			}
		})
	}
}

func TestConcreteDocumentLineCoverageAndEventPolicy(t *testing.T) {
	source := "\ufeff[Script Info]\r\nTitle:  A, B\r\n\r\n[Events]\nFormat: Start, Text, End\n" +
		"Dialogue: 0:00,Hello, world,0:02\r\nComment: raw,untouched\r" +
		"unrecognized final line"
	document := ParseConcreteDocument(source)
	if document.Source != source {
		t.Fatal("document source changed")
	}
	offset := 0
	var dialogueRecord *ConcreteRecord
	for _, line := range document.Lines {
		if line.Span.Start != offset || source[line.Span.Start:line.Span.End] != line.Raw ||
			line.Content != source[line.ContentSpan.Start:line.ContentSpan.End] ||
			line.Terminator != source[line.ContentSpan.End:line.Span.End] {
			t.Fatalf("line source coverage at %d: %#v", offset, line)
		}
		if line.Header != nil && source[line.Header.NameSpan.Start:line.Header.NameSpan.End] != line.Header.RawName {
			t.Fatalf("header source range: %#v", line.Header)
		}
		if line.Record != nil {
			record := line.Record
			if source[record.KeySpan.Start:record.KeySpan.End] != record.Key ||
				source[record.BodySpan.Start:record.BodySpan.End] != record.Body ||
				source[record.Colon] != ':' {
				t.Fatalf("record source range: %#v", record)
			}
			if strings.TrimSpace(record.Key) == "Dialogue" {
				dialogueRecord = record
				if line.Section != "Events" {
					t.Fatalf("section = %q", line.Section)
				}
			}
		}
		offset = line.Span.End
	}
	if offset != len(source) || dialogueRecord == nil {
		t.Fatalf("source coverage ended at %d (want %d), record = %#v", offset, len(source), dialogueRecord)
	}

	// Profiles can choose different tail-consuming fields without changing raw
	// separators or assuming commas in subtitle text end event fields.
	first := dialogueRecord.Partition(source, 3, 1)
	second := dialogueRecord.Partition(source, 3, 2)
	if len(first) != 3 || len(second) != 3 ||
		first[1].Raw != "Hello, world,0:02" ||
		second[1].Raw != "Hello" || second[2].Raw != " world,0:02" {
		t.Fatalf("renderer-driven field partitions = %#v / %#v", first, second)
	}
	if dialogueRecord.Body != " 0:00,Hello, world,0:02" {
		t.Fatalf("leading whitespace normalized: %q", dialogueRecord.Body)
	}
}

func TestConcreteEventPartitionMissingAndEmptyFields(t *testing.T) {
	source := "Dialogue: A,,C"
	record := ParseConcreteDocument(source).Lines[0].Record
	cells := record.Partition(source, 5, -1)
	if len(cells) != 5 || cells[0].Raw != " A" || cells[1].Raw != "" ||
		!cells[1].Present || cells[2].Raw != "C" ||
		cells[3].Present || cells[4].Present {
		t.Fatalf("empty vs missing fields = %#v", cells)
	}
	if cells[1].Delimiter < 0 || cells[3].Span.Start != len(source) {
		t.Fatalf("separator/missing offsets lost: %#v", cells)
	}
}

func TestConcreteDocumentEncodingFixturesAndDialogueCoordinateMapping(t *testing.T) {
	for _, name := range []string{
		"ordinary.ass", "renderer-collisions.ass", "utf16le.ass", "utf8bom.ass",
	} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile(filepath.Join("..", "..", "testdata", "contracts", "cases", name))
			if err != nil {
				t.Fatal(err)
			}
			source, err := DecodeSource(input)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(source.Encode(source.Text), input) {
				t.Fatal("BOM or original source encoding changed during round-trip")
			}
			concrete := ParseConcreteDocument(source.Text)
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
				tree := ParseConcreteDialogue(dialogue.Text)
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

func FuzzConcreteDocumentLineCoverage(f *testing.F) {
	for _, source := range []string{
		"", "[Events]\r\nDialogue: a,b\r\n", "X:Y,,,", "a\rb\nc\r\nd", "é\n日本語",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 32768 {
			t.Skip()
		}
		document := ParseConcreteDocument(source)
		cursor := 0
		for _, line := range document.Lines {
			if line.Span.Start != cursor || line.Raw != source[cursor:line.Span.End] ||
				line.Content != source[line.ContentSpan.Start:line.ContentSpan.End] ||
				line.Terminator != source[line.ContentSpan.End:line.Span.End] {
				t.Fatalf("invalid line span: %#v", line)
			}
			if strings.ContainsAny(line.Content, "\r\n") {
				t.Fatalf("physical newline retained in line content: %#v", line)
			}
			switch line.Terminator {
			case "", "\r", "\n", "\r\n":
			default:
				t.Fatalf("invalid physical line terminator: %#v", line)
			}
			cursor = line.Span.End
		}
		if cursor != len(source) {
			t.Fatalf("lost %d bytes", len(source)-cursor)
		}
	})
}

func TestConcretePartitionTracksTrailingDelimitedEmptyFields(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   []bool
	}{
		{"Dialogue:", []bool{false, false, false}},
		{"Dialogue: A,", []bool{true, true, false}},
		{"Dialogue: A,,", []bool{true, true, true}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			record := ParseConcreteDocument(tc.source).Lines[0].Record
			cells := record.Partition(tc.source, 3, -1)
			for i, want := range tc.want {
				if cells[i].Present != want {
					t.Fatalf("cell %d Present = %v, want %v: %#v", i, cells[i].Present, want, cells)
				}
			}
		})
	}
}
