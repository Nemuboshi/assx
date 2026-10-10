# Differential renderer compatibility (P07/10)

Implementation: [Issue #22](https://github.com/Nemuboshi/assx/issues/22) of
[Epic #16](https://github.com/Nemuboshi/assx/issues/16). This builds on P04
profiles, P05 independent evaluation, and P06 renderer-local validity.

## API boundary

`semantic.CompareDialogue(concreteDialogue, profiles, options)` and
`semantic.CompareDocument(concreteDocument, profiles)` return typed pairwise
observations. `lint.AnalyzeCompatibility(dialogue, profiles)` and
`lint.AnalyzeDocumentCompatibility(document, profiles)` turn findings into
separate diagnostics. An empty target list selects libass + xy-VSFilter;
VSFilterMod is explicit and retains its `_VSMOD`/`_LUA` build capabilities.
Duplicate profiles are removed without changing caller order.

The default CLI and existing `Analyze`/`AnalyzeDocument` APIs retain their
frozen diagnostics, JSON, exit behavior, and fixes. Compatibility APIs provide
no edits or fix safety classification. Renderer selection in the CLI belongs
to P10; equivalence proofs for edits belong to P08.

Syntax stays in `internal/ass`; source partitioning and renderer evidence stay
in `internal/ass/renderer`; comparison of effective operations and scenario
selection stay in `internal/semantic`; rule selection and diagnostic rendering
stay in `internal/lint`. The formatter can reuse semantic findings without
importing lint.

## What a result means

Each finding names a source span, renderer pair, dimension, status, both
interpretations, explanation, and available citations. Statuses are:

| Status | Meaning within the named dimension |
| --- | --- |
| `equivalent` | Verified observations agree for this dimension only. |
| `divergent` | Independently interpreted, source-backed observations differ. |
| `ignored-or-unsupported` | Both profiles ignore or do not support the construct; this does not prove useful rendering. |
| `unresolved` | A missing model, inferred evidence, malformed expression, or unknown build prevents a conclusion. |

Dimensions distinguish dispatch, arguments, accepted/rejected signatures,
application, first-wins ownership, explicit behavior scenarios, tracked state,
and document record layouts. Multiple failures at one span survive diagnostic
deduplication. Findings are deterministic in source and requested target order.
`ASS003` reports proven differences; `ASS031` reports unresolved or ignored
compatibility observations. The diagnostic `field` identifies the dimension
and `renderer` identifies both pinned profiles and their build capabilities.
These findings are separate from renderer-local validity diagnostics.

Each profile resolves and evaluates the same neutral source independently.
Observer data is detached before retention and shared as read-only snapshots
across dimensions, so a finding does not repeatedly copy a large state event.
Nested operations have separate observations rather than duplicated recursive
child views. Ownership refers to original
source spans, not tag indices that can differ between profile operation
streams. Nested constructs absent from one stream remain unresolved.

Verified signatures establish argument shapes, not complete state semantics.
Application conclusions require accepted/rejected signatures and certain
transitions. Operations with first-wins ownership additionally require verified
repeat behavior; ordinary assignments do not depend on latch evidence. Equal
tracked state **never establishes rendering equivalence**. A known state
difference can be reported when those proof gates hold, but state agreement
remains unresolved. Even
an equivalent dispatch or documented behavior scenario does not guarantee
pixel-identical output.

## Evidence and reference cases

`docs/ass-tags.xml` explicitly scopes verified scenario cells. The generated
matrix now retains those rows alongside signatures. Missing rows do not inherit
editorial scenario defaults, `status="S"` and unverified renderer cells cannot
prove equivalence, and unknown builds cannot borrow an enabled branch's proof.
Scenario `requires` metadata guards the VSFilterMod outcome even when the
command itself is core, such as the optional argument-consuming `fsc` handler.
Disabled branches stay unresolved unless separately verified.
Scenario candidates come from both interpretations. Each candidate's domain is
checked independently on each side before its pinned outcomes are compared;
reversing the renderer pair preserves dimensions and statuses. Rectangular
rounding requires four fully parsed numeric coordinates on both sides, while
empty-component normalization is a parser-level observation independent of
accepted arity. A repeat outcome requires an occupied latch on each side.
Missing or inapplicable evidence remains unresolved.
Distinct labels alone do not prove an
invocation difference: `zero` versus `restored` can produce the same value,
and an integer relative `fs` argument does not reveal fractional truncation.
Reset mechanism differences therefore remain unresolved; the font-size parser
comparison requires exact fractional input.

- `\fsvp6` selects traditional `\fs` with argument `vp6`, versus Mod's
  extension `\fsvp` with argument `6`. Dispatch diverges; the extension's
  inferred signature and unmodeled effects remain unresolved.
- `\frs10` similarly selects traditional `\fr` versus Mod's `\frs`.
- `\pos(1,2,3)` is source-verified rejected by traditional profiles and
  accepted by Mod with `_VSMOD` enabled. With a later `\pos(4,5)`, ownership
  and known position state differ. An unknown Mod build stays unresolved.
- `\pos(1,,2)` discards the empty component in all three pinned parsers.
  Each consumes two arguments and can claim the position latch; a subsequent
  valid position has the same source-aligned first owner. The shared resolver
  normalizes parameters while retaining every surviving raw argument's
  original source span.
- `\blend(add)` reaches Mod's **blend** branch when enabled. Issue #22's
  statement that `be` shadows it contradicts the pinned source: `blend`
  begins with `bl`, not `be`. Traditional profiles select `b`; parenthesized
  prefix/suffix consumption and unmodeled extension state stay unresolved.
  See Mod `RTS.cpp:2548-2557,3716-3730` at
  `7a00567e4a49b6310691b9a6791646b2a018bfa2` and the existing matrix note.
- Verified scenario rows expose blur limits, legacy alignment mapping,
  rectangular clip rounding, charset effects, and reversed move windows.
  For example, `\be200` remains unresolved across traditional profiles
  because the relevant xy scenario cell is unverified.
- `\clip(1.5,,0,10,10)` and `\iclip(1.5,,0,10,10)` become verified
  rectangles in both traditional parsers after empty-component removal, so
  their rounding difference remains visible in either profile order. A true
  five-argument shape or malformed/partially parsed coordinates cannot borrow
  the rounding proof. Conversion overflow and exponent forms that Mod's
  `wcstol` does not fully consume also remain unresolved.

## Document and Style scope

Profiles independently partition ASS v4+ Style and Dialogue records from the
lossless document. libass honors Format names; xy-VSFilter and VSFilterMod
consume fixed field positions. Actor/Name aliases are normalized. Custom
Style or event field order therefore produces a source-backed layout
comparison instead of inheriting one common legacy parse.

Keyword spelling also follows the profile: libass ignores nonexact `Format:`
record keywords while the VSFilter loaders ignore Format records altogether.
Each profile's own Text span is evaluated with Style inputs derived from its
own record layout. Document observations retain absolute decoded-source spans;
lint maps them to physical lines, including CR, LF, and CRLF. Records dropped
by the historical parser remain visible to compatibility analysis.

This API verifies source partitioning, not full renderer document loading.
Unsupported dialects, malformed framing, Style value conversion/fallback,
unresolved resets, drawing, karaoke, and time-dependent transforms remain
explicitly unresolved where no sufficient model/evidence exists. Shared Style
canonicalization is an input to the partial evaluator and cannot authorize
Style or pixel equivalence. This boundary avoids duplicating a complete
renderer document loader during P07.

## Validation

```sh
go test ./... -count=1
go test -race ./internal/semantic ./internal/lint ./internal/ass/renderer -count=1
go vet ./...
go build ./cmd/assx
(cd internal/ass/renderer && go run ./cmd/genmatrix -check)
go test ./internal/semantic -run '^$' -bench '^BenchmarkCompareDialogue$' -benchmem
```

Regression tests cover independent prefix dispatch, signature acceptance,
first-wins ownership and state, empty-component consumption, verified and
unverified scenarios, unknown builds, nested transforms, malformed clips,
resets, drawing, karaoke, detached observations, source spans, deduplication,
custom document layouts, physical lines, and the absence of compatibility
fixes. All three pinned parsers' empty-component removal and the first-wins
arity differences are source-pinned in the matrix. Profile-order, invocation
domain, scalar-application, build-guard, and mechanism-label regressions
prevent unsupported scenario conclusions.

On Windows amd64, the one-iteration `long_10000` compatibility benchmark's
cumulative allocation fell from 1,627,355,952 to 368,625,184 bytes (77.3%)
when dimensions began sharing observations. The plain-dialogue fixture now
allocates zero bytes in comparison. These are allocation measurements, not
peak-memory measurements or statistically established timing improvements.
Existing default benchmarks were compared against `main@5efedc6` with three
100 ms runs on the same machine; most allocation counts were unchanged and
timing was noisy. The compatibility path remains opt-in.

Existing frozen default contracts remain unchanged.
