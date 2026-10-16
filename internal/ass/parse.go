package ass

import (
	"strings"
	"unicode"
)

// Parse projects the lossless concrete syntax tree onto the renderer-facing
// document view. Line framing follows every physical newline form (LF, CRLF,
// bare CR), so the view agrees with the renderers about the file's shape.
// The concrete tree stays the authority for source spans and raw bytes.
func Parse(text string) Document {
	tree := ParseConcreteDocument(text)
	doc := Document{Text: text, Headers: map[string]HeaderField{}, Newline: lineTerminator(text)}
	section := ""
	var styleFormat, eventFormat []string
	for i := range tree.Lines {
		line := tree.Lines[i]
		doc.Lines = append(doc.Lines, RawLine{
			Line: line.Number, Offset: line.Span.Start, Content: line.Content, Section: section,
		})
		if line.Header != nil {
			section = strings.ToLower(strings.TrimSpace(line.Header.RawName))
			styleFormat = nil
			if section == "script info" && doc.ScriptInfoLine == 0 {
				doc.ScriptInfoLine = line.Number
				doc.ScriptInfoInsert = line.Span.End
			}
			continue
		}
		record := line.Record
		if record == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(record.Key))
		switch {
		case section == "script info":
			switch key {
			case "playresx", "playresy", "ycbcr matrix", "layoutresx", "layoutresy":
				if _, exists := doc.Headers[key]; exists {
					continue
				}
				value, start, end := trimmedBody(record.Body)
				doc.Headers[key] = HeaderField{
					Value: value, Line: line.Number,
					ValueStart: record.BodySpan.Start + start, ValueEnd: record.BodySpan.Start + end,
				}
			}
		case isStyleSection(section):
			if key == "format" {
				styleFormat = ParseFormat(record.Body)
				continue
			}
			if key != "style" {
				continue
			}
			if styleFormat == nil {
				styleFormat = defaultStyleFormat(section)
			}
			doc.StyleFields = append(doc.StyleFields, parseStyleFields(line.Content, line.Span.Start, line.Number, styleFormat)...)
		case section == "events":
			if key == "format" {
				doc.EventFormat = ParseFormat(record.Body)
				eventFormat = doc.EventFormat
				continue
			}
			if key != "dialogue" {
				continue
			}
			format := eventFormat
			if len(format) == 0 {
				// libass substitutes the standard v4+ event format
				// (ass.c event_format_fallback) when no Format line was
				// read yet.
				format = standardEventFormat
			}
			// A Format line without a Text name silently drops every
			// event in libass; the Format line itself gets flagged.
			if !hasFormatName(format, "text") {
				continue
			}
			prefix := strings.Index(strings.ToLower(line.Content), "dialogue:")
			bodyStart := prefix + len("Dialogue:")
			body := strings.TrimLeft(line.Content[bodyStart:], " \t")
			bodyStart = len(line.Content) - len(body)
			doc.Dialogues = append(doc.Dialogues, parseEventLine(body, line.Span.Start+bodyStart, line.Span.Start, line.Number, format))
		}
	}
	return doc
}

func isStyleSection(section string) bool {
	return section == "v4+ styles" || section == "v4 styles" || section == "v4++ styles"
}

// defaultStyleFormat mirrors the fixed Style columns libass assumes before a
// Format line is read. Only the two dialects libass defines have defaults.
func defaultStyleFormat(section string) []string {
	if section == "v4 styles" {
		return ParseFormat("Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, AlphaLevel, Encoding")
	}
	return ParseFormat("Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding")
}

// lineTerminator picks the newline spelling for inserted header lines. It
// prefers CRLF, then a bare CR in a CR-only file, and defaults to LF.
func lineTerminator(text string) string {
	if strings.Contains(text, "\r\n") {
		return "\r\n"
	}
	if !strings.Contains(text, "\n") && strings.Contains(text, "\r") {
		return "\r"
	}
	return "\n"
}

// trimmedBody returns a record body without surrounding whitespace and the
// byte offsets of the trimmed value inside the body.
func trimmedBody(body string) (value string, start, end int) {
	left := strings.TrimLeftFunc(body, unicode.IsSpace)
	start = len(body) - len(left)
	value = strings.TrimRightFunc(left, unicode.IsSpace)
	return value, start, start + len(value)
}

// standardEventFormat mirrors libass ass_event_format (ass.c:49-50), used
// when an Events section has no Format line.
var standardEventFormat = ParseFormat("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text")

// HasFormatName reports whether a Format list contains the lowercased field
// name.
func HasFormatName(format []string, name string) bool {
	return hasFormatName(format, name)
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
		if missingAt >= 0 && (name != "text" || cursor != len(body)+1) {
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
