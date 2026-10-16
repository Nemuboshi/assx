package ass

import (
	"reflect"
	"testing"
)

// A physical line break is a line break in every renderer. libass splits on
// both CR and LF (process_text, libass/ass.c:1211-1234), so the document view
// must not depend on the newline spelling.
func TestParseLineFramingIsNewlineIndependent(t *testing.T) {
	const lf = "[Script Info]\nScriptType: v4.00+\nPlayResX: 640\nYCbCr Matrix: None\n" +
		"[V4+ Styles]\nFormat: Name, Fontname, Alignment\nStyle: Default,Arial,2\n" +
		"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
		"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,{\\fs20}x\n"
	crlf := replaceNewlines(lf, "\r\n")
	cr := replaceNewlines(lf, "\r")

	reference := Parse(lf)
	if len(reference.Dialogues) != 1 || len(reference.StyleFields) != 3 || len(reference.Lines) != 10 {
		t.Fatalf("LF reference view is wrong: dialogues=%d styles=%d lines=%d",
			len(reference.Dialogues), len(reference.StyleFields), len(reference.Lines))
	}
	for name, text := range map[string]string{"CRLF": crlf, "CR": cr} {
		got := Parse(text)
		if len(got.Dialogues) != 1 || len(got.StyleFields) != 3 || len(got.Lines) != 10 {
			t.Fatalf("%s view lost lines: dialogues=%d styles=%d lines=%d",
				name, len(got.Dialogues), len(got.StyleFields), len(got.Lines))
		}
		if got.Dialogues[0].Text != reference.Dialogues[0].Text {
			t.Fatalf("%s event text = %q, want %q", name, got.Dialogues[0].Text, reference.Dialogues[0].Text)
		}
		// The text offset is relative to its own line, which is newline-independent.
		if want := reference.Dialogues[0].TextStart - reference.Dialogues[0].LineStart; got.Dialogues[0].TextStart-got.Dialogues[0].LineStart != want {
			t.Fatalf("%s event text column offset = %d, want %d", name, got.Dialogues[0].TextStart-got.Dialogues[0].LineStart, want)
		}
		if !reflect.DeepEqual(headerValues(got), headerValues(reference)) {
			t.Fatalf("%s headers = %v, want %v", name, headerValues(got), headerValues(reference))
		}
		if !reflect.DeepEqual(got.EventFormat, reference.EventFormat) {
			t.Fatalf("%s event format = %v, want %v", name, got.EventFormat, reference.EventFormat)
		}
	}
}

func TestParseNewlinePrefersCRLFFromSource(t *testing.T) {
	for text, want := range map[string]string{
		"a\r\nb\r\n": "\r\n",
		"a\nb\n":     "\n",
		"a\rb\r":     "\r",
		"":           "\n",
	} {
		if got := Parse(text).Newline; got != want {
			t.Fatalf("Parse(%q).Newline = %q, want %q", text, got, want)
		}
	}
}

func headerValues(doc Document) map[string]string {
	values := make(map[string]string, len(doc.Headers))
	for name, field := range doc.Headers {
		values[name] = field.Value
	}
	return values
}

func replaceNewlines(text, newline string) string {
	out := make([]byte, 0, len(text))
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			out = append(out, newline...)
			continue
		}
		out = append(out, text[i])
	}
	return string(out)
}
