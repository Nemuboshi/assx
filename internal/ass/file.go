package ass

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"
)

// EventField keeps one Format-named event field with its raw source span.
// Value is the exact source text between separators; span offsets are byte
// offsets into Document.Text, so fixes and diagnostics keep raw data.
type EventField struct {
	Name  string // lowercased Format name
	Value string
	Start int
	End   int
}

type Dialogue struct {
	Text        string
	Style       string
	StyleColumn int
	Line        int
	LineStart   int
	TextStart   int
	Fields      []EventField
	// MissingFields marks a line that ran out of values before the Format
	// names did. libass discards such events (process_event_tail breaks on a
	// null token and returns failure), and xy-VSFilter throws in NextInt and
	// rejects the whole line.
	MissingFields bool
}

// Field returns the first event field whose Format name is name.
func (d Dialogue) Field(name string) (EventField, bool) {
	for _, field := range d.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return EventField{}, false
}

type HeaderField struct {
	Value      string
	Line       int
	ValueStart int
	ValueEnd   int
}

type StyleField struct {
	StyleName   string
	Name        string
	Value       string
	Line        int
	ValueStart  int
	ValueEnd    int
	ValueColumn int
}

// RawLine keeps one physical line with its byte offset and the section it
// appeared in, so document-level checks can reason about syntax libass drops.
type RawLine struct {
	Line    int
	Offset  int
	Content string
	Section string
}

type Document struct {
	Text             string
	Dialogues        []Dialogue
	StyleFields      []StyleField
	Headers          map[string]HeaderField
	Lines            []RawLine
	EventFormat      []string
	ScriptInfoLine   int
	ScriptInfoInsert int
	Newline          string
}

type Source struct {
	Text         string
	bom          []byte
	utf16        bool
	littleEndian bool
}

func DecodeSource(data []byte) (Source, error) {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		little := data[0] == 0xff
		if (len(data)-2)%2 != 0 {
			return Source{}, errors.New("UTF-16 input has an incomplete code unit")
		}
		words := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i < len(data); i += 2 {
			if little {
				words = append(words, binary.LittleEndian.Uint16(data[i:i+2]))
			} else {
				words = append(words, binary.BigEndian.Uint16(data[i:i+2]))
			}
		}
		return Source{Text: string(utf16.Decode(words)), bom: append([]byte(nil), data[:2]...), utf16: true, littleEndian: little}, nil
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		return Source{Text: string(data[3:]), bom: append([]byte(nil), data[:3]...)}, nil
	}
	return Source{Text: string(data)}, nil
}

func (s Source) Encode(text string) []byte {
	if !s.utf16 {
		return append(append([]byte(nil), s.bom...), []byte(text)...)
	}
	words := utf16.Encode([]rune(text))
	out := make([]byte, len(s.bom)+2*len(words))
	copy(out, s.bom)
	for i, word := range words {
		if s.littleEndian {
			binary.LittleEndian.PutUint16(out[len(s.bom)+2*i:], word)
		} else {
			binary.BigEndian.PutUint16(out[len(s.bom)+2*i:], word)
		}
	}
	return out
}

func hasFormatName(format []string, name string) bool {
	for _, field := range format {
		if field == name {
			return true
		}
	}
	return false
}

// ParseFormat returns the normalized field names in a Style or Events Format line.
func ParseFormat(format string) []string {
	columns := strings.Split(format, ",")
	for i := range columns {
		columns[i] = strings.ToLower(strings.TrimSpace(columns[i]))
	}
	return columns
}
