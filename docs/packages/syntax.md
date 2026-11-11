# Lossless renderer-neutral syntax

## Current contract

`ParseConcreteDialogue` is the only dialogue syntax parser.
It does not consult `spec.TagSpecs` or select a renderer.
Each node and item retains an exact half-open span and raw source bytes.
Slash runs, heads, commas, parentheses, unknown content, and incomplete expressions survive intact.

A `ConcreteExpression` is a lexical candidate, not a resolved tag.
The head `fsvp6` does not establish whether the command is `fsvp` or `fs`.
A three-coordinate `pos` survives parsing without becoming valid in the primary renderer.
`renderer.Profile` resolves each candidate independently.
The generic `renderer.Tag` is a resolved decoder and observer view, not another parser. Decode helpers live in `internal/ass/renderer`, so `ass` keeps only the physical tree.
There is no `ParseDialogueText`, inferred `DecodeTag`, token adapter, or legacy tag resolver.

Parenthesis traversal is iterative.
`ConcreteExpression.Components(source)` exposes source-backed comma components and nested candidates lazily.
`WalkConcreteExpressions` visits lexical expressions for source inspection.
`Profile.WalkDialogue` visits nested commands only when the parent resolves as a transform.
Syntax traversal alone cannot authorize an edit.

## Document syntax

`ParseConcreteDocument` retains physical lines, terminators, headers, records, and comma offsets.
`ConcreteRecord.Partition` requires an explicit field count and optional tail-consuming field.
Absent cells differ from delimited empty cells.
Renderer profiles own field layout interpretation; the CST does not infer a dialect.

`ass.ParseSource` parses the physical structure once and keeps source coordinates.
It uses LF-delimited framing. The concrete document preserves standalone CR, LF, and CRLF.
`renderer.DefaultDocument` supplies the default libass view from that tree.
`renderer.Prepare` supplies every requested view from one default view, and
`renderer.Profile.View` supplies one profile's view. Neither changes the tree.
Compatibility analysis independently partitions the concrete document under each requested profile.
Unknown dialects and malformed framing remain unresolved.

## Coordinates and encoding

All syntax spans refer to decoded UTF-8 byte offsets in a Go string.
Dialogue spans are text-relative. Document spans are document-relative.
`ConcreteSpan.AtDocumentOffset` maps between these coordinate systems.
`DecodeSource` removes BOMs and decodes UTF-16 before parsing.
`Source.Encode` restores the original encoding and BOM.
SafeFix requires byte-exact round-trip equality before replacing the file.

`internal/sourcepos` indexes decoded UTF-8 text for LSP positions. It maps byte spans to zero-based UTF-16 lines and characters. It rejects offsets inside UTF-8 runes or CRLF and positions inside surrogate pairs. A final newline starts an empty LSP line. Raw file byte offsets do not enter this index.

## Input recognition

`ass.RecognizeParsed` classifies decoded content before CLI analysis. It uses the physical lines and section context from an existing `ass.Parsed`. It does not use the filename suffix, lint rules, or a renderer profile. Its evidence contains stable identifiers and physical line numbers.

`Recognized` requires a credible record inside `[Events]`, a Style section, or `[Script Info]`. A meaningful event or Style `Format:` also qualifies. A `Comment:` record alone stays `Tentative`. A known section without credible records is `Tentative`. Text without credible ASS structure is `Rejected`. A NUL or other disallowed control byte rejects the input. A WebVTT marker or subtitle timing arrow rejects sparse input. Credible ASS records take precedence over foreign-looking text elsewhere. Unknown sections and malformed ASS values do not cause rejection by themselves.

The CLI returns exit 2 for rejected input before lint or font checks. It permits lint of tentative input but does not write fixes. It does not bypass source decoding, byte-exact round-trip protection, or fix proof. Input notices go to stderr; JSON diagnostics stay on stdout. Recognition is an input policy, not a semantic validity rule.

See [reports](../usage/diagnostic-reports.md) for display coordinates and
[contracts](../rules/verification.md) for reviewed snapshots and fix evidence.
