package ass

import "strings"

// ConcreteDocument retains the exact decoded source, physical lines, section
// headers and colon records without assigning renderer-dependent event fields.
type ConcreteDocument struct {
	Source string
	Lines  []ConcreteLine
}

type ConcreteLine struct {
	Number      int
	Span        ConcreteSpan // includes the line terminator, when present
	ContentSpan ConcreteSpan // excludes the terminator
	Raw         string
	Content     string
	Terminator  string
	Section     string // unmodified section name in effect before this line
	Header      *ConcreteSection
	Record      *ConcreteRecord
}

type ConcreteSection struct {
	NameSpan ConcreteSpan
	RawName  string
}

type ConcreteRecord struct {
	KeySpan  ConcreteSpan
	BodySpan ConcreteSpan
	Key      string
	Body     string
	Colon    int   // absolute decoded-source byte offset
	Commas   []int // ALL commas; no implicit Text-column or Format policy
}

// ConcreteCell is one field slice produced under an explicitly supplied
// separator policy. Absent slots retain an empty zero-width source span.
type ConcreteCell struct {
	Span      ConcreteSpan
	Raw       string
	Present   bool
	Delimiter int // absolute comma byte offset; -1 for a terminal cell
}

// Partition makes no claim about which column is Text. A renderer resolver
// supplies the number of fields and the index of its tail-consuming field;
// use tailColumn=-1 for records where every comma is a separator. The record
// and original document source are never mutated.
func (r ConcreteRecord) Partition(source string, count, tailColumn int) []ConcreteCell {
	if count <= 0 {
		return nil
	}
	cells := make([]ConcreteCell, 0, count)
	cursor := r.BodySpan.Start
	commaIndex := 0
	exhausted := false
	for i := 0; i < count; i++ {
		if exhausted {
			cells = append(cells, ConcreteCell{
				Span: ConcreteSpan{r.BodySpan.End, r.BodySpan.End}, Delimiter: -1,
			})
			continue
		}
		if i == tailColumn {
			cells = append(cells, ConcreteCell{
				Span: ConcreteSpan{cursor, r.BodySpan.End},
				Raw:  source[cursor:r.BodySpan.End], Present: true, Delimiter: -1,
			})
			exhausted = true
			continue
		}
		if commaIndex < len(r.Commas) {
			comma := r.Commas[commaIndex]
			cells = append(cells, ConcreteCell{
				Span: ConcreteSpan{cursor, comma},
				Raw:  source[cursor:comma], Present: true, Delimiter: comma,
			})
			cursor = comma + 1
			commaIndex++
			continue
		}
		cells = append(cells, ConcreteCell{
			Span:      ConcreteSpan{cursor, r.BodySpan.End},
			Raw:       source[cursor:r.BodySpan.End],
			Present:   cursor < r.BodySpan.End || (commaIndex > 0 && r.Commas[commaIndex-1] == cursor-1),
			Delimiter: -1,
		})
		exhausted = true
	}
	return cells
}

// ParseConcreteDocument preserves the physical representation of every line,
// including blank lines, CRLF, bare CR, final lines without a terminator, and
// unrecognized record types. Section names and keys are not lowercased.
func ParseConcreteDocument(source string) ConcreteDocument {
	document := ConcreteDocument{Source: source}
	section := ""
	number := 0
	walkConcreteLines(source, func(start, end, contentEnd int) {
		number++
		line := ConcreteLine{
			Number:      number,
			Span:        ConcreteSpan{start, end},
			ContentSpan: ConcreteSpan{start, contentEnd},
			Raw:         source[start:end], Content: source[start:contentEnd],
			Terminator: source[contentEnd:end], Section: section,
		}
		trimmed := strings.TrimSpace(line.Content)
		if len(trimmed) >= 2 && trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']' {
			leading := strings.Index(line.Content, trimmed)
			nameStart := start + leading + 1
			nameEnd := nameStart + len(trimmed) - 2
			line.Header = &ConcreteSection{
				NameSpan: ConcreteSpan{nameStart, nameEnd},
				RawName:  source[nameStart:nameEnd],
			}
			section = line.Header.RawName
		} else if colon := strings.IndexByte(line.Content, ':'); colon >= 0 {
			absoluteColon := start + colon
			bodyStart := absoluteColon + 1
			record := &ConcreteRecord{
				KeySpan:  ConcreteSpan{start, absoluteColon},
				BodySpan: ConcreteSpan{bodyStart, contentEnd},
				Key:      source[start:absoluteColon], Body: source[bodyStart:contentEnd],
				Colon: absoluteColon,
			}
			for pos := bodyStart; pos < contentEnd; pos++ {
				if source[pos] == ',' {
					record.Commas = append(record.Commas, pos)
				}
			}
			line.Record = record
		}
		document.Lines = append(document.Lines, line)
	})
	return document
}

// walkConcreteLines owns the shared no-allocation physical-line framing.
// The default document parser can consume it without allocating CST records.
func walkConcreteLines(source string, visit func(start, end, contentEnd int)) {
	for start := 0; start < len(source); {
		end := len(source)
		if newline := strings.IndexByte(source[start:], '\n'); newline >= 0 {
			end = start + newline + 1
		}
		contentEnd := end
		if source[contentEnd-1] == '\n' {
			contentEnd--
		}
		if contentEnd > start && source[contentEnd-1] == '\r' {
			contentEnd--
		}
		visit(start, end, contentEnd)
		start = end
	}
}
