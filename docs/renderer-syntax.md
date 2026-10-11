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
The generic `ass.Tag` is a resolved decoder and observer view, not another parser.
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

`ass.Parse` supplies the existing document lint view and source coordinates.
It uses LF-delimited framing. The concrete document preserves standalone CR, LF, and CRLF.
Compatibility analysis independently partitions the concrete document under each requested profile.
Unknown dialects and malformed framing remain unresolved.

## Coordinates and encoding

All syntax spans refer to decoded UTF-8 byte offsets in a Go string.
Dialogue spans are text-relative. Document spans are document-relative.
`ConcreteSpan.AtDocumentOffset` maps between these coordinate systems.
`DecodeSource` removes BOMs and decodes UTF-16 before parsing.
`Source.Encode` restores the original encoding and BOM.
SafeFix requires byte-exact round-trip equality before replacing the file.

See [reports](diagnostic-reports.md) for display coordinates and
[contracts](renderer-contracts.md) for reviewed snapshots and fix evidence.

## Implementation record: P02/P03

P02/P03 introduced the concrete syntax tree beside a temporary legacy adapter.
P10 removed that adapter and its duplicate dialogue parsing and semantic paths.
Earlier unchanged-default claims describe the migration stages, not the current contract.
Historical benchmark samples remain in [performance](performance.md).
