package ass

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDialogueTextOffsetsAndHeaderValues(t *testing.T) {
	text := "[Script Info]\r\nPlayResX: 1920\r\nPlayResY: 1080\r\nYCbCr Matrix: TV.601\r\n[Events]\r\nFormat: Layer, Start, End, Text\r\nDialogue: 0,0:00:00.00,0:00:01.00,hello, world\r\n"
	doc := Parse(text)
	if len(doc.Dialogues) != 1 {
		t.Fatalf("got %d dialogue rows, want 1", len(doc.Dialogues))
	}
	dialogue := doc.Dialogues[0]
	if dialogue.Text != "hello, world" || dialogue.Line != 7 {
		t.Fatalf("dialogue = %#v, want text with comma on physical line 7", dialogue)
	}
	if got := doc.Text[dialogue.TextStart : dialogue.TextStart+len(dialogue.Text)]; got != dialogue.Text {
		t.Fatalf("text span = %q, want %q", got, dialogue.Text)
	}
	matrix := doc.Headers["ycbcr matrix"]
	if matrix.Value != "TV.601" || text[matrix.ValueStart:matrix.ValueEnd] != matrix.Value {
		t.Fatalf("matrix field = %#v, span %q", matrix, text[matrix.ValueStart:matrix.ValueEnd])
	}
	if doc.ScriptInfoLine != 1 || text[doc.ScriptInfoInsert:doc.ScriptInfoInsert+len("PlayResX")] != "PlayResX" {
		t.Fatalf("Script Info insertion point = line %d, offset %d", doc.ScriptInfoLine, doc.ScriptInfoInsert)
	}
}

func TestParseDialogueStyleColumn(t *testing.T) {
	line := "Dialogue: 0,0,1,  *default  ,hello"
	doc := Parse("[Events]\nFormat: Layer, Start, End, Style, Text\n" + line + "\n")
	if len(doc.Dialogues) != 1 {
		t.Fatalf("got %d dialogue rows, want 1", len(doc.Dialogues))
	}
	dialogue := doc.Dialogues[0]
	wantColumn := strings.Index(line, "*default") + 1
	if dialogue.Style != "*default" || dialogue.StyleColumn != wantColumn {
		t.Fatalf("dialogue style location = %q at column %d, want %q at column %d", dialogue.Style, dialogue.StyleColumn, "*default", wantColumn)
	}
}

func TestParseEventFieldsWithRawSpans(t *testing.T) {
	text := "[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\nDialogue: 12,0:01:02.03,0:02:03.04,Default,Actor,10,20,30,Banner;5,{\\i1}hi\n"
	doc := Parse(text)
	if len(doc.Dialogues) != 1 {
		t.Fatalf("dialogues = %d, want 1", len(doc.Dialogues))
	}
	dialogue := doc.Dialogues[0]
	if dialogue.MissingFields {
		t.Fatalf("complete event marked missing")
	}
	for _, name := range []string{"layer", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"} {
		field, ok := dialogue.Field(name)
		if !ok {
			t.Fatalf("field %q absent from %#v", name, dialogue.Fields)
		}
		if got := text[field.Start:field.End]; got != field.Value {
			t.Fatalf("field %q span %q != value %q", name, got, field.Value)
		}
	}
	layer, _ := dialogue.Field("layer")
	if layer.Value != "12" {
		t.Fatalf("layer = %q", layer.Value)
	}
	effect, _ := dialogue.Field("effect")
	if effect.Value != "Banner;5" {
		t.Fatalf("effect = %q, want raw value up to the last separator", effect.Value)
	}
	if dialogue.Text != `{\i1}hi` || dialogue.Style != "Default" {
		t.Fatalf("text = %q style = %q", dialogue.Text, dialogue.Style)
	}
	if dialogue.LineStart == 0 || !strings.HasPrefix(text[dialogue.LineStart:], "Dialogue:") {
		t.Fatalf("line start offset = %d", dialogue.LineStart)
	}
}

func TestParseEventFieldsKeepRawSpansAndTrailingCells(t *testing.T) {
	text := "[Events]\nFormat: Layer, Start, End, Text\nDialogue: 0,0,1,{\\i1}a,b\n"
	dialogue := Parse(text).Dialogues[0]
	if dialogue.Text != `{\i1}a,b` {
		t.Fatalf("text = %q, want remainder after the third separator", dialogue.Text)
	}
	start, _ := dialogue.Field("start")
	if got := text[start.Start:start.End]; got != start.Value {
		t.Fatalf("start span = %q, value = %q", got, start.Value)
	}
}

func TestParseEventFieldsDetectMissingFields(t *testing.T) {
	text := "[Events]\nFormat: Layer, Start, End, Style, Text\nDialogue: 0,0:00:01.00,0:00:02.00\n"
	doc := Parse(text)
	if len(doc.Dialogues) != 1 {
		t.Fatalf("parser must keep the raw event for linting: %v", doc.Dialogues)
	}
	dialogue := doc.Dialogues[0]
	if !dialogue.MissingFields {
		t.Fatalf("short event not marked missing")
	}
	if _, ok := dialogue.Field("style"); !ok {
		t.Fatalf("style field must still be listed, missing value: %#v", dialogue.Fields)
	}
	if dialogue.Text != "" {
		t.Fatalf("text = %q, want empty", dialogue.Text)
	}
}

func TestParseEventFieldsWithoutTextFormatDropsEvents(t *testing.T) {
	text := "[Events]\nFormat: Layer, Start, End, Style\nDialogue: 0,0,1,Default\n"
	doc := Parse(text)
	if len(doc.Dialogues) != 0 {
		t.Fatalf("libass discards events whose format has no Text: %#v", doc.Dialogues)
	}
	if hasFormatName(doc.EventFormat, "text") {
		t.Fatal("format parsed without Text must stay inspectable")
	}
}

func TestParseEventFieldsFallbackWithoutFormatLine(t *testing.T) {
	text := "[Events]\nDialogue: 0,0:00:01.00,0:00:02.00,Default,Actor,0,0,0,,hi\n"
	doc := Parse(text)
	if len(doc.Dialogues) != 1 {
		t.Fatalf("missing Format must fall back to the standard v4+ format: %v", doc.Dialogues)
	}
	dialogue := doc.Dialogues[0]
	if dialogue.Text != "hi" || dialogue.Style != "Default" {
		t.Fatalf("fallback parse = %#v", dialogue)
	}
}

func TestParseEventFieldsReorderedFormat(t *testing.T) {
	text := "[Events]\nFormat: Text, Layer, Start, End, Style\nDialogue: hi,0,0:00:01.00,0:00:02.00,Default\n"
	dialogue := Parse(text).Dialogues[0]
	// libass stops at the first Format name "Text" and assigns the whole
	// remainder to it, while xy-VSFilter keeps its fixed positions: the
	// reordered line is a divergence the Format rule must flag.
	if dialogue.Text != "hi,0,0:00:01.00,0:00:02.00,Default" {
		t.Fatalf("text = %q, want the whole remainder", dialogue.Text)
	}
	if _, ok := dialogue.Field("style"); ok {
		t.Fatalf("fields after Text must not be parsed: %#v", dialogue.Fields)
	}
}

func TestDecodeSourceRoundTripsBOMEncodings(t *testing.T) {
	cases := map[string][]byte{
		"utf8-bom": append([]byte{0xef, 0xbb, 0xbf}, []byte("[Script Info]\n")...),
		"utf16-le": {0xff, 0xfe, '[', 0, 'A', 0},
		"utf16-be": {0xfe, 0xff, 0, '[', 0, 'A'},
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			source, err := DecodeSource(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := source.Encode(source.Text); !reflect.DeepEqual(got, raw) {
				t.Fatalf("round trip = %v, want %v", got, raw)
			}
		})
	}
}

func TestDecodeRejectsIncompleteUTF16(t *testing.T) {
	if _, err := DecodeSource([]byte{0xff, 0xfe, 0x41}); err == nil {
		t.Fatal("expected incomplete UTF-16 code unit error")
	}
}
