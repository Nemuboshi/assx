# Renderer-scoped effective semantics (P05/10)

Implementation: [Issue #20](https://github.com/Nemuboshi/assx/issues/20) in
[Epic #16](https://github.com/Nemuboshi/assx/issues/16). This builds on the
independent syntax and source-backed dispatch delivered in P02–P04.

## Evaluation boundary

A single state engine in `internal/semantic` evaluates commands for one
renderer at a time. `EvaluateResolved` consumes a renderer-neutral
`ass.ConcreteDialogue` and a pinned `renderer.Profile`; it never assigns
tag names in the CST. The `EvaluationOptions.Profile` path also supports
existing consumers that only hold a `DialogueText`, re-lexing the original
source into the neutral CST rather than using the historical tag names.

The adapter in `operation.go` produces a sequence of text boundaries and
resolved operations. Each operation retains source coordinates, its
renderer-selected command, argument bytes, profile policy, match status,
signature evidence, and prefix-shadowing information. Transform children
are resolved from their original nested expressions and evaluated in source
order. A single evaluator owns StateView, style lookup, precedence,
first-wins latches, provenance, liveness, and proof revocation.

`DecodeTagWithSpec` is the typed scalar decoder entry point when a command
has already been resolved. Semantic canonicalization, assignment handlers,
and transform comparison receive this policy explicitly, without looking
up a second, potentially different tag identity.

## Known state and uncertainty

Match status, verified argument shapes, and modeled state are separate:

- A verified, applicable signature permits semantic interpretation; it does
  not by itself establish known state or edit equivalence.
- A source-verified rejected form (currently the exhaustive `pos` shape)
  is ignored by that renderer and cannot acquire a first-wins latch.
- A build-conditional or unresolved shape invalidates affected values and
  proofs. When it could have claimed a first-wins slot, a subsequent valid
  assignment cannot fabricate a definite owner.
- A definite first-wins owner survives later conditional operations that
  could not displace it.
- Recognized but unmodeled extensions are barriers rather than invented
  state transitions. Name-unknown or explicitly ignored commands remain
  distinguishable in TagEvent resolution metadata.
- Malformed clip shapes invalidate both rectangle and vector clipping
  values and provenance, including after resets. They never establish
  a *definite* vector owner.
- Transform uncertainty and rendering-time variation are represented
  separately. NoEffect proof revocation is retroactive.

For example, with `{\pos(1,2,3)\pos(4,5)}`, libass and xy-VSFilter
reject the first form and can apply the second. VSFilterMod with `_VSMOD`
enabled can apply the first and ignore the second. A build with unknown
capabilities cannot prove which command owns the position slot.

## Consumers and rollout

This section records the P05 implementation. Use the linked subject documents
for later phases.

The default `Evaluate` entry point uses a compatibility input adapter.
It shares the state engine with renderer-scoped evaluation.
The adapter does not prove equivalence across renderers.
Its removal requires CLI migration and baseline review.

Consumers can now evaluate explicit profiles:

- `lint.AnalyzeNoEffectsForRenderer` for ASS006 observations;
- `lint.AnalyzeRedundantStyleOverridesForRenderer` for ASS013;
- `lint.AnalyzeFontsForRenderer` for font state and drawing visibility.

These observations carry no automatic edits. See [scoped lint](renderer-lint.md)
for validity diagnostics, [compatibility](renderer-compatibility.md) for renderer
comparison, and [contracts](renderer-contracts.md) for SafeFix proof scope.
An unmodeled extension remains a proof barrier.

The existing CLI, JSON schema, diagnostic order, and default SafeFix
classification are deliberately unchanged in P05. No new renderer CLI
selector is introduced.

## Validation

Use the checked-in contract golden files; do not regenerate a mismatch
without independent source evidence.

```sh
go test ./... -count=1
go test -race ./internal/semantic ./internal/lint -count=1
go vet ./...
go build ./cmd/assx
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -count=1
go test ./internal/semantic -run '^TestRendererScoped' -count=1
go test ./internal/lint -run '^TestRendererScoped' -count=1
go test ./internal/semantic -run '^$' -bench '^BenchmarkEvaluate(Resolved|Dialogue)' -benchmem
```

The tests cover first-wins divergence, build capability uncertainty,
prefix shadowing, nested transform resolution, clip provenance, style
resets, font consumers, and the absence of single-profile SafeFix edits.
