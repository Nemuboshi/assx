// Package sourcepos converts decoded UTF-8 byte offsets to LSP UTF-16 positions.
package sourcepos

import (
	"fmt"
	"sort"
	"unicode/utf8"

	"assx/internal/edit"
)

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type physicalLine struct {
	start, contentEnd int
}

type Index struct {
	text  string
	lines []physicalLine
}

// New indexes decoded text. A final newline starts an empty final LSP line.
func New(text string) (Index, error) {
	if !utf8.ValidString(text) {
		return Index{}, fmt.Errorf("source text is not valid UTF-8")
	}
	index := Index{text: text}
	for start := 0; start < len(text); {
		end := start
		for end < len(text) && text[end] != '\r' && text[end] != '\n' {
			end++
		}
		index.lines = append(index.lines, physicalLine{start: start, contentEnd: end})
		if end == len(text) {
			break
		}
		if text[end] == '\r' && end+1 < len(text) && text[end+1] == '\n' {
			end++
		}
		start = end + 1
	}
	if len(index.lines) == 0 || index.lines[len(index.lines)-1].contentEnd < len(text) {
		index.lines = append(index.lines, physicalLine{start: len(text), contentEnd: len(text)})
	}
	return index, nil
}

// Position maps a byte boundary to a UTF-16 position. A CRLF interior is not a boundary.
func (index Index) Position(offset int) (Position, error) {
	if len(index.lines) == 0 {
		return Position{}, fmt.Errorf("source position index is not initialized")
	}
	if offset < 0 || offset > len(index.text) {
		return Position{}, fmt.Errorf("byte offset %d is outside the source", offset)
	}
	lineNumber := sort.Search(len(index.lines), func(i int) bool { return index.lines[i].start > offset }) - 1
	line := index.lines[lineNumber]
	if offset > line.contentEnd {
		return Position{}, fmt.Errorf("byte offset %d is inside a line terminator", offset)
	}
	segment := index.text[line.start:offset]
	if !utf8.ValidString(segment) {
		return Position{}, fmt.Errorf("byte offset %d splits a UTF-8 rune", offset)
	}
	character := 0
	for _, r := range segment {
		character++
		if r > 0xffff {
			character++
		}
	}
	return Position{Line: lineNumber, Character: character}, nil
}

// Offset rejects a position inside a surrogate pair or past the line content.
func (index Index) Offset(position Position) (int, error) {
	if len(index.lines) == 0 {
		return 0, fmt.Errorf("source position index is not initialized")
	}
	if position.Line < 0 || position.Line >= len(index.lines) || position.Character < 0 {
		return 0, fmt.Errorf("invalid UTF-16 position %+v", position)
	}
	line := index.lines[position.Line]
	character := 0
	for offset := line.start; offset < line.contentEnd; {
		if character == position.Character {
			return offset, nil
		}
		r, size := utf8.DecodeRuneInString(index.text[offset:line.contentEnd])
		character++
		if r > 0xffff {
			character++
		}
		offset += size
		if character > position.Character {
			return 0, fmt.Errorf("UTF-16 position %+v splits a surrogate pair", position)
		}
	}
	if character == position.Character {
		return line.contentEnd, nil
	}
	return 0, fmt.Errorf("UTF-16 position %+v is past the line", position)
}

func (index Index) Range(start, end int) (Range, error) {
	if end < start {
		return Range{}, fmt.Errorf("reversed byte range [%d:%d]", start, end)
	}
	first, err := index.Position(start)
	if err != nil {
		return Range{}, err
	}
	last, err := index.Position(end)
	if err != nil {
		return Range{}, err
	}
	return Range{Start: first, End: last}, nil
}

func (index Index) EditRange(change edit.TextEdit) (Range, error) {
	if !utf8.ValidString(change.Replacement) {
		return Range{}, fmt.Errorf("edit replacement is not valid UTF-8")
	}
	return index.Range(change.Start, change.End)
}
