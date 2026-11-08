# Source edits

## Current contract

`edit.TextEdit` holds a half-open range and a replacement:
`Start`, `End`, and `Replacement`. Offsets are decoded UTF-8 byte offsets in
the source string. A `Replacement` may be empty. The type carries no severity,
rule ID, or authority.

`edit.Interval` holds the same half-open range. `Interval.Overlaps` reports
whether two ranges intersect. It exists for deterministic ordering and for
overlap detection before an edit group reaches the text.

`edit.Apply(text, edits)` sorts a copy of the edits by start offset, then by end
offset, and writes the result into a new string. It rejects a negative offset, a
reversed range, a range past the end of the text, and an edit whose start is
inside the previous edit. It never mutates the input and never reads a file.

The package does not decide which edits are correct. `internal/lint` builds a
candidate edit group and `internal/analysis` applies it after proof. An edit
group is applied whole or not at all.

## Coordinates

Every span in `internal/ass`, `internal/semantic`, and `internal/lint` is a
decoded UTF-8 byte offset. `edit` uses the same offsets, so a span reaches an
edit without conversion. Editor positions use
[source positions](sourcepos.md) instead.

## Validation

`go test ./internal/edit` covers overlapping ranges, reversed ranges, ordering
by start offset, and an empty edit list. `internal/lint` and
`internal/analysis` tests cover the proof gate that precedes application.
