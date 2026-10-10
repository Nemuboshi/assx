# Renderer-neutral syntax migration

Parent: #16. Implementation: #18 (P02 and P03).

## Source ownership

`ParseConcreteDialogue` constructs a **lossless concrete syntax view** without
consulting `spec.TagSpecs`. Every node and item has an exact, half-open byte
span into the original decoded dialogue string. The complete slash run,
uninterpreted head, parentheses, top-level comma positions, malformed suffixes,
unknown text and unclosed constructs survive intact.

A `ConcreteExpression` is a lexical **candidate**, not a tag. For example,
the exact head `fsvp6` can later be interpreted as `fsvp` with argument `6`
or as `fs` with an implementation-specific tail. Neither choice is committed
in the syntax tree. Similarly, a three-coordinate `pos` is preserved without
implying it is valid for the default target.

Parenthesis nesting is lexical and iterative. Inner slashes stay inside their
parenthesized container until renderer resolution chooses how to consume them.
Incomplete parentheses and trailing unmatched braces are never discarded.

## Compatibility during the epic

`ParseDialogueText` retains the public-internal `DialogueText`, `Tag`,
`BlockItem` and `WalkTokens` interfaces used by lint, font analysis, the
semantic engine and SafeFix. It now gets top-level block framing from the
neutral syntax parser, while `legacy_tags.go` explicitly owns the historical
default tag resolver. The compatibility view remains an intentional migration
boundary, not an alternative normative tag grammar.

The default is deliberately **unchanged** until the profile-specific resolver
and semantic engine land in P04/P05. Source spans, diagnostics, SafeFix and
`Dialogue.Syntax` caching are pinned by the P01 regression contract. New
renderer tags must never be added to the global legacy resolver merely because
another renderer recognizes them.

## Next steps

P03 adds lazy structural inspection of nested parenthesized expressions and a
source-preserving document/event view, with the same decoded UTF-8 byte
coordinate system. P04/P05 will resolve concrete candidates independently per
pinned renderer and remove the compatibility resolver when migration is
complete. No syntax-only evidence can authorize a SafeFix.


## P03: parenthesized and document syntax

`ConcreteExpression.Components(source)` exposes its comma-delimited top-level
components lazily, preserving every component's exact byte span. Each component
also has source-backed raw items and lexical candidates. For example,
`\\t(0,100,\\clip(0,0,10,10)\\pos(1,2,3))` retains the nested clips,
transform arguments and full source coordinates. This structural inspection is
available for ANY parenthesized expression; it does not assert that the
renderer recognizes a transform or that its children are valid tags.

`ParseConcreteDocument` returns physical lines with exact terminators,
unmodified section headers, unknown records and all raw comma offsets.
`ConcreteRecord.Partition` requires the caller to specify an explicit field
count and optional tail-consuming field, so the syntax layer does not bake
in libass/xy-VSFilter/VSFilterMod field acceptance or `Text` semantics.
Missing slots remain distinguishable from delimited empty slots.

Both document parsing and the historical `Parse` share one allocation-free
physical-line scanner with **explicit framing policies**. The neutral CST
recognizes standalone CR, LF and CRLF as distinct source-preserving line
terminators, including in mixed-line-ending files. The historical default
`Parse` deliberately keeps its LF-delimited behavior (including its existing
handling of CR before LF or at EOF), so enabling richer syntax does not
silently alter existing line numbers, Format mapping, diagnostics or edits.
The legacy line policy can be reconsidered in a later explicitly tested
behavior-change PR. Renderer-specific field acceptance belongs in the later
profile resolver, not in `ConcreteDocument`.

Field partitioning preserves the distinction between an **absent** cell and
an **explicitly delimited empty** cell, including a tail-consuming Text field.
Presence requires source content or a delimiter directly preceding an empty
cell; a missing terminal field is never manufactured by the tail policy.

### Coordinates and encoding

All syntax spans are half-open byte offsets in the **decoded Go string**.
An event field offset is document-relative, whereas a dialogue syntax offset
is relative to that dialogue's text. `ConcreteSpan.AtDocumentOffset` maps
the latter to the former without changing the edit coordinate system.

For UTF-8 with BOM or UTF-16 inputs, `DecodeSource` removes the BOM and
decodes UTF-16 before parsing. Therefore syntax spans refer to decoded UTF-8
bytes, not the positions of UTF-16 code units in the input file.
`Source.Encode` restores the original encoding and BOM when writing source.
P01 fixture tests freeze the byte-for-byte round trip, including CRLF.

Both P02 and P03 deliberately keep renderer-neutral syntax separate from the
legacy default interpretation; do not introduce renderer-specific name
prefixes or argument normalization here. Later migration PRs must preserve
the golden default contract and remove the legacy resolver only after a
verified renderer-aware replacement exists.

## Non-regression performance note

The P01 pinned baseline (`main@6ba7453`) and P02/P03 runs were measured
in the same Windows/amd64 benchmark environment with Go 1.27.0. The implementation runs
used `-benchmem -count=3` with 100 ms (dialogue) and 200 ms (document)
benchtimes, versus 250 ms and 5 samples in P01. Hence the short-run timings
are not used as a CI performance gate; allocation metrics are reproducible.

| Default parse fixture | P01 B/op | P02/P03 B/op | P01 allocs/op | P02/P03 allocs/op |
| --- | ---: | ---: | ---: | ---: |
| `BenchmarkParseDialogueTextFixtures/typesetting` | 96,960 | 96,960 | 1,020 | 1,020 |
| `BenchmarkASSDocumentParse/typesetting` | 255,032 | 253,882 | 1,496 | 1,495 |

No additional concrete document or nested candidate tree is eagerly
materialized by existing default lint, font or semantic analysis.
