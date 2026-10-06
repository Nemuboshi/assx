package ass

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"unicode/utf16"
)

type Dialogue struct {
	Text      string
	Style     string
	Line      int
	TextStart int
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

type Document struct {
	Text             string
	Dialogues        []Dialogue
	StyleFields      []StyleField
	Headers          map[string]HeaderField
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
	section, textColumn, styleColumn := "", 9, 3
	var styleFormat []string
	offset, line := 0, 0
	for _, chunk := range strings.SplitAfter(text, "\n") {
		if chunk == "" {
			continue
		}
		line++
		raw := strings.TrimSuffix(chunk, "\n")
		content := strings.TrimSuffix(raw, "\r")
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
				columns := strings.Split(trimmed[strings.IndexByte(trimmed, ':')+1:], ",")
				styleFormat = make([]string, len(columns))
				for i, column := range columns {
					styleFormat[i] = strings.ToLower(strings.TrimSpace(column))
				}
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
				columns := strings.Split(strings.TrimSpace(trimmed[len("Format:"):]), ",")
				for i, column := range columns {
					name := strings.TrimSpace(column)
					if strings.EqualFold(name, "Text") {
						textColumn = i
					}
					if strings.EqualFold(name, "Style") {
						styleColumn = i
					}
				}
			} else if strings.HasPrefix(lower, "dialogue:") {
				prefix := strings.Index(strings.ToLower(content), "dialogue:")
				bodyStart := prefix + len("Dialogue:")
				body := strings.TrimLeft(content[bodyStart:], " \t")
				bodyStart = len(content) - len(body)
				separator := -1
				for i := 0; i < textColumn; i++ {
					next := strings.IndexByte(body[separator+1:], ',')
					if next < 0 {
						separator = -1
						break
					}
					separator += next + 1
				}
				if separator >= 0 {
					style := ""
					if styleColumn < textColumn {
						fields := strings.Split(body[:separator], ",")
						if styleColumn < len(fields) {
							style = strings.TrimSpace(fields[styleColumn])
						}
					}
					textStart := bodyStart + separator + 1
					doc.Dialogues = append(doc.Dialogues, Dialogue{Text: body[separator+1:], Style: style, Line: line, TextStart: offset + textStart})
				}
			}
		}
		offset += len(chunk)
	}
	return doc
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
