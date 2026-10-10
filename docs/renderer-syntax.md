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
