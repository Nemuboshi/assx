# Source positions

## Current contract

`sourcepos.New(text)` indexes decoded UTF-8 text into physical lines. It rejects
invalid UTF-8. A CRLF terminator counts as one terminator. A final newline starts
an empty final line.

`Index.Position(offset)` maps a byte boundary to a zero-based UTF-16 line and
character. It rejects an offset outside the text, an offset inside a UTF-8 rune,
and an offset inside a line terminator.

`Index.Offset(position)` maps a UTF-16 position back to a byte offset. It rejects
a position past the line content and a position inside a surrogate pair. A
character after an astral-plane rune counts as two UTF-16 units.

`Index.Range(start, end)` maps a byte range to two positions. It rejects a
reversed range. `Index.EditRange(change)` validates that the replacement is
valid UTF-8, then maps the edit range.

## Boundaries

This package is the only place that converts to and from LSP UTF-16 positions.
Span consumers keep decoded UTF-8 byte offsets.

The index covers one document text. It does not accept raw file byte offsets and
does not know about an encoding. BOM removal and UTF-16 decoding happen earlier,
in `ass.DecodeSource`.

An error is a protocol boundary, not a diagnostic. The language server catches
it, reports it on its log, and drops that position instead of publishing a
diagnostic with a wrong range.

## Validation

`go test ./internal/sourcepos` covers CR, LF, and CRLF framing, a final newline,
a tab, an astral-plane rune, offsets inside a rune, offsets inside a terminator,
positions inside a surrogate pair, positions past a line, reversed ranges,
insertion ranges, an empty document, a decoded BOM, and invalid UTF-8. The LSP
tests cover publishing a diagnostic range for a document that has a version.
