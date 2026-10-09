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
	Syntax      DialogueText
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

func (d Dialogue) ParsedText() DialogueText {
	if d.Syntax.Source == d.Text && (d.Text == "" || d.Syntax.Nodes != nil) {
		return d.Syntax
	}
	return ParseDialogueText(d.Text)
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

func Parse(text string) Document {
	doc := Document{Text: text, Headers: map[string]HeaderField{}, Newline: "\n"}
	if strings.Contains(text, "\r\n") {
		doc.Newline = "\r\n"
	}
	section := ""
	var styleFormat []string
	offset, line := 0, 0
	for _, chunk := range strings.SplitAfter(text, "\n") {
		if chunk == "" {
			continue
		}
		line++
		raw := strings.TrimSuffix(chunk, "\n")
		content := strings.TrimSuffix(raw, "\r")
		doc.Lines = append(doc.Lines, RawLine{Line: line, Offset: offset, Content: content, Section: section})
		trimmed := strings.TrimSpace(content)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.ToLower(trimmed[1 : len(trimmed)-1])
			styleFormat = nil
			if section == "v4+ styles" {
				styleFormat = splitFormat("Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding")
			} else if section == "v4 styles" {
				styleFormat = splitFormat("Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, AlphaLevel, Encoding")
			}
			if section == "script info" && doc.ScriptInfoLine == 0 {
				doc.ScriptInfoLine = line
				doc.ScriptInfoInsert = offset + len(chunk)
			}
			offset += len(chunk)
			continue
		}
		if section == "v4+ styles" || section == "v4 styles" || section == "v4++ styles" {
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "format:") {
				styleFormat = splitFormat(trimmed[strings.IndexByte(trimmed, ':')+1:])
			} else if strings.HasPrefix(lower, "style:") && len(styleFormat) > 0 {
				doc.StyleFields = append(doc.StyleFields, parseStyleFields(content, offset, line, styleFormat)...)
			}
		}
		if section == "script info" {
			if strings.HasPrefix(strings.ToLower(trimmed), "playresx:") || strings.HasPrefix(strings.ToLower(trimmed), "playresy:") || strings.HasPrefix(strings.ToLower(trimmed), "ycbcr matrix:") || strings.HasPrefix(strings.ToLower(trimmed), "layoutresx:") || strings.HasPrefix(strings.ToLower(trimmed), "layoutresy:") {
				colon := strings.IndexByte(content, ':')
				name := strings.ToLower(strings.TrimSpace(content[:colon]))
				valueRaw := content[colon+1:]
				leading := len(valueRaw) - len(strings.TrimLeft(valueRaw, " \t"))
				value := strings.TrimSpace(valueRaw)
				valueStart := offset + colon + 1 + leading
				valueEnd := offset + colon + 1 + len(strings.TrimRight(valueRaw, " \t"))
				if _, exists := doc.Headers[name]; !exists {
					doc.Headers[name] = HeaderField{Value: value, Line: line, ValueStart: valueStart, ValueEnd: valueEnd}
				}
			}
		}
		if section == "events" {
			lower := strings.ToLower(trimmed)
			if strings.HasPrefix(lower, "format:") {
				doc.EventFormat = splitFormat(trimmed[strings.IndexByte(trimmed, ':')+1:])
			} else if strings.HasPrefix(lower, "dialogue:") {
				format := doc.EventFormat
				if len(format) == 0 {
					// libass substitutes the standard v4+ event format
					// (ass.c event_format_fallback) when no Format line was
					// read yet.
					format = standardEventFormat
				}
				// A Format line without a Text name silently drops every
				// event in libass; the Format line itself gets flagged.
				if hasFormatName(format, "text") {
					prefix := strings.Index(strings.ToLower(content), "dialogue:")
					bodyStart := prefix + len("Dialogue:")
					body := strings.TrimLeft(content[bodyStart:], " \t")
					bodyStart = len(content) - len(body)
					doc.Dialogues = append(doc.Dialogues, parseEventLine(body, offset+bodyStart, offset, line, format))
				}
			}
		}
		offset += len(chunk)
	}
	return doc
}

// standardEventFormat mirrors libass ass_event_format (ass.c:49-50), used
// when an Events section has no Format line.
var standardEventFormat = splitFormat("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text")

// HasFormatName reports whether a Format list contains the lowercased field
// name.
func HasFormatName(format []string, name string) bool {
	return hasFormatName(format, name)
}

func hasFormatName(format []string, name string) bool {
	for _, field := range format {
		if field == name {
			return true
		}
	}
	return false
}

// parseEventLine maps each Format name onto the Dialogue body and keeps the
// raw span of every field. libass reads cells by Format name and discards the
// event when the line runs out before the names do (ass.c next_token returns
// NULL at end-of-string, process_event_tail then breaks and frees the event);
// xy-VSFilter reads the same cells at fixed positions and throws in NextInt
// on a missing integer cell (STS.cpp:1284-1301). "Text" consumes the whole
// remainder, like both renderers.
func parseEventLine(body string, bodyOffset, lineOffset, line int, format []string) Dialogue {
	dialogue := Dialogue{Line: line, LineStart: lineOffset}
	missingAt := -1
	cursor := 0
	for i, name := range format {
		if missingAt < 0 && (cursor > len(body) || (cursor == len(body) && !strings.HasSuffix(body, ","))) {
			// libass next_token returns NULL at end-of-string for every
			// remaining name, including Text, and the event is discarded.
			missingAt = i
		}
		if missingAt >= 0 && !(name == "text" && cursor == len(body)+1) {
			dialogue.Fields = append(dialogue.Fields, EventField{Name: name, Start: bodyOffset + len(body), End: bodyOffset + len(body)})
			continue
		}
		if name == "text" {
			value := ""
			if cursor < len(body) {
				value = body[cursor:]
			}
			field := EventField{Name: name, Value: value, Start: bodyOffset + cursor, End: bodyOffset + len(body)}
			dialogue.Text = value
			dialogue.TextStart = bodyOffset + cursor
			dialogue.Syntax = ParseDialogueText(dialogue.Text)
			dialogue.Fields = append(dialogue.Fields, field)
			break
		}
		end := len(body)
		if comma := strings.IndexByte(body[cursor:], ','); comma >= 0 {
			end = cursor + comma
		}
		value := body[cursor:end]
		field := EventField{Name: name, Value: value, Start: bodyOffset + cursor, End: bodyOffset + end}
		if name == "style" {
			dialogue.Style = strings.TrimSpace(value)
			leading := len(value) - len(strings.TrimLeft(value, " \t"))
			dialogue.StyleColumn = field.Start - lineOffset + leading + 1
		}
		dialogue.Fields = append(dialogue.Fields, field)
		cursor = end + 1
	}
	dialogue.MissingFields = missingAt >= 0
	return dialogue
}

func splitFormat(format string) []string {
	columns := strings.Split(format, ",")
	for i := range columns {
		columns[i] = strings.ToLower(strings.TrimSpace(columns[i]))
	}
	return columns
}

func parseStyleFields(content string, offset, line int, format []string) []StyleField {
	prefix := strings.Index(strings.ToLower(content), "style:")
	if prefix < 0 {
		return nil
	}
	bodyStart := prefix + len("Style:")
	body := strings.TrimLeft(content[bodyStart:], " \t")
	bodyStart = len(content) - len(body)
	var values []StyleField
	fieldStart := 0
	styleName := ""
	for i, name := range format {
		fieldEnd := len(body)
		if i < len(format)-1 {
			comma := strings.IndexByte(body[fieldStart:], ',')
			if comma < 0 {
				break
			}
			fieldEnd = fieldStart + comma
		}
		raw := body[fieldStart:fieldEnd]
		leading := len(raw) - len(strings.TrimLeft(raw, " \t"))
		value := strings.TrimSpace(raw)
		start := offset + bodyStart + fieldStart + leading
		end := offset + bodyStart + fieldStart + len(strings.TrimRight(raw, " \t"))
		if name == "name" {
			styleName = value
		}
		values = append(values, StyleField{StyleName: styleName, Name: name, Value: value, Line: line, ValueStart: start, ValueEnd: end, ValueColumn: bodyStart + fieldStart + leading + 1})
		fieldStart = fieldEnd + 1
	}
	for i := range values {
		values[i].StyleName = styleName
	}
	return values
}
