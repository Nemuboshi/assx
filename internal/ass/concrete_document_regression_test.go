package ass

import (
	"reflect"
	"strings"
	"testing"
)

func TestConcreteDocumentMixedLineTerminators(t *testing.T) {
	source := "[Events]\rDialogue: 0,hello\r\nComment: x\rLast"
	doc := ParseConcreteDocument(source)
	if len(doc.Lines) != 4 {
		t.Fatalf("got %d lines, want 4: %#v", len(doc.Lines), doc.Lines)
	}
	wantContents := []string{"[Events]", "Dialogue: 0,hello", "Comment: x", "Last"}
	wantTerminators := []string{"\r", "\r\n", "\r", ""}
	wantSections := []string{"", "Events", "Events", "Events"}
	cursor := 0
	for i, line := range doc.Lines {
		if line.Number != i+1 || line.Span.Start != cursor ||
			line.Content != wantContents[i] || line.Terminator != wantTerminators[i] ||
			line.Section != wantSections[i] ||
			source[line.Span.Start:line.Span.End] != line.Raw ||
			source[line.ContentSpan.Start:line.ContentSpan.End] != line.Content ||
			source[line.ContentSpan.End:line.Span.End] != line.Terminator {
			t.Fatalf("line %d has incorrect content, section, or spans: %#v", i+1, line)
		}
		cursor = line.Span.End
	}
	if cursor != len(source) {
		t.Fatalf("ended at %d, want %d", cursor, len(source))
	}
	if doc.Lines[0].Header == nil || doc.Lines[0].Header.RawName != "Events" {
		t.Fatalf("lost section header: %#v", doc.Lines[0])
	}
	if doc.Lines[1].Record == nil || doc.Lines[1].Record.Key != "Dialogue" ||
		doc.Lines[1].Record.Body != " 0,hello" ||
		doc.Lines[2].Record == nil || doc.Lines[2].Record.Key != "Comment" {
		t.Fatalf("lost records: %#v", doc.Lines)
	}
}

func TestConcreteDocumentAllLineEndingForms(t *testing.T) {
	tests := []struct {
		source   string
		contents []string
		endings  []string
	}{
		{"", nil, nil},
		{"hello", []string{"hello"}, []string{""}},
		{"one\rtwo\r", []string{"one", "two"}, []string{"\r", "\r"}},
		{"one\ntwo\n", []string{"one", "two"}, []string{"\n", "\n"}},
		{"one\r\ntwo\r\n", []string{"one", "two"}, []string{"\r\n", "\r\n"}},
		{"\r\n\r\n", []string{"", ""}, []string{"\r\n", "\r\n"}},
		{"\r\n\r\n\r", []string{"", "", ""}, []string{"\r\n", "\r\n", "\r"}},
		{"A\r\rB\n\nC\r\nD\rE", []string{"A", "", "B", "", "C", "D", "E"}, []string{"\r", "\r", "\n", "\n", "\r\n", "\r", ""}},
	}
	for _, tt := range tests {
		t.Run(strings.ReplaceAll(strings.ReplaceAll(tt.source, "\r", "CR"), "\n", "LF"), func(t *testing.T) {
			doc := ParseConcreteDocument(tt.source)
			if len(doc.Lines) != len(tt.contents) {
				t.Fatalf("got %d lines, want %d: %#v", len(doc.Lines), len(tt.contents), doc.Lines)
			}
			var assembled strings.Builder
			at := 0
			for i, line := range doc.Lines {
				if line.Content != tt.contents[i] || line.Terminator != tt.endings[i] ||
					line.Span != (ConcreteSpan{at, at + len(line.Raw)}) ||
					line.ContentSpan != (ConcreteSpan{at, at + len(line.Content)}) {
					t.Fatalf("line %d: %#v", i, line)
				}
				assembled.WriteString(line.Raw)
				at += len(line.Raw)
			}
			if assembled.String() != tt.source || at != len(tt.source) {
				t.Fatalf("source round trip failed: %q", assembled.String())
			}
		})
	}
}

func TestLegacyDocumentLineBehaviorUnaffectedByCROnlyInput(t *testing.T) {
	inputs := []string{
		"[Events]\rDialogue: 0,hello\r\nComment: x\rLast",
		"[Events]\rFormat: Layer, Start, End, Style, Text\rDialogue: 0,0,1,Default,text\r",
		"[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0,1,Default,{\\pos(1,2)}x\r",
		"[Events]\r\nFormat: Layer, Start, End, Style, Text\r\nDialogue: 0,0,1,Default,x\r\n",
	}
	for _, source := range inputs {
		t.Run(source, func(t *testing.T) {
			got := Parse(source)
			chunks := strings.SplitAfter(source, "\n")
			var want []RawLine
			offset, number, section := 0, 0, ""
			for _, chunk := range chunks {
				if chunk == "" {
					continue
				}
				number++
				content := strings.TrimSuffix(strings.TrimSuffix(chunk, "\n"), "\r")
				want = append(want, RawLine{Line: number, Offset: offset, Content: content, Section: section})
				trimmed := strings.TrimSpace(content)
				if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
					section = strings.ToLower(trimmed[1 : len(trimmed)-1])
				}
				offset += len(chunk)
			}
			if !reflect.DeepEqual(got.Lines, want) {
				t.Fatalf("changed historical LF-only framing\ngot: %#v\nwant: %#v", got.Lines, want)
			}
			if strings.Contains(source, "\rFormat:") && len(got.Dialogues) != 0 {
				t.Fatalf("legacy parser unexpectedly began recognizing CR-only events: %#v", got.Dialogues)
			}
		})
	}
}

func TestConcreteTailCellPresenceDependsOnSourceEvidence(t *testing.T) {
	tests := []struct {
		source      string
		count, tail int
		raw         []string
		present     []bool
	}{
		{"Dialogue:", 1, 0, []string{""}, []bool{false}},
		{"Dialogue:", 3, 2, []string{"", "", ""}, []bool{false, false, false}},
		{"Dialogue:,", 2, 1, []string{"", ""}, []bool{true, true}},
		{"Dialogue:A,", 2, 1, []string{"A", ""}, []bool{true, true}},
		{"Dialogue:A", 2, 1, []string{"A", ""}, []bool{true, false}},
		{"Dialogue:A,B", 3, 2, []string{"A", "B", ""}, []bool{true, true, false}},
		{"Dialogue:A,B,", 3, 2, []string{"A", "B", ""}, []bool{true, true, true}},
		{"Dialogue:A,B,C,D", 3, 2, []string{"A", "B", "C,D"}, []bool{true, true, true}},
		{"Dialogue:,,", 3, 2, []string{"", "", ""}, []bool{true, true, true}},
		{"Dialogue: ", 1, 0, []string{" "}, []bool{true}},
	}
	for _, tc := range tests {
		t.Run(tc.source+"/"+strings.Repeat("t", tc.tail+1), func(t *testing.T) {
			record := ParseConcreteDocument(tc.source).Lines[0].Record
			cells := record.Partition(tc.source, tc.count, tc.tail)
			if len(cells) != tc.count {
				t.Fatalf("got %d cells, want %d", len(cells), tc.count)
			}
			for i, c := range cells {
				if c.Raw != tc.raw[i] || c.Present != tc.present[i] ||
					tc.source[c.Span.Start:c.Span.End] != c.Raw {
					t.Fatalf("cell %d = %#v, want raw %q / present %v", i, c, tc.raw[i], tc.present[i])
				}
			}
		})
	}
}
