package ass

import (
	"reflect"
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
