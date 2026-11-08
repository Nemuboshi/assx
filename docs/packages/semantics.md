# Renderer-scoped effective semantics

## Evaluation boundary

It uses the independent syntax and source-backed dispatch.
A single state engine in `internal/semantic` evaluates commands for one
renderer at a time. `EvaluateResolved` consumes a renderer-neutral
`ass.ConcreteDialogue` and a pinned `renderer.Profile`; it never assigns
tag names in the CST. There is no nil-profile or legacy evaluation path.
Every caller supplies a concrete tree and explicit pinned profile.

The adapter in `operation.go` produces a sequence of text boundaries and
resolved operations. Each operation retains source coordinates, its
renderer-selected command, argument bytes, profile policy, match status,
signature evidence, and prefix-shadowing information. Transform children
are resolved from their original nested expressions and evaluated in source
order. A single evaluator owns StateView, style lookup, precedence,
first-wins latches, provenance, liveness, and proof revocation.
Named resets retain the original Dialogue Style and consume the first resolved argument.
Unknown named Styles remain proof barriers; no missing Style is invented.

`DecodeTagWithSpec` is the typed scalar decoder entry point when a command
has already been resolved. Semantic canonicalization, assignment handlers,
and transform comparison receive this policy explicitly, without looking
up a second, potentially different tag identity.

## Evaluation API

`TagSpec` owns the static `SemanticKind` dispatch policy. Declared value kinds,
affected slots, and precedence rules cover generic assignments. A small set of
special forms uses `internal/semantic/handlers.go`: transforms, clipping, Style
resets, karaoke, drawing mode, and relative font sizes. `TransformComparable`
declares the transform targets eligible for no-effect comparison without a
separate tag-name whitelist.

`Observer.Tag` receives a `TagEvent` after each tag transition: assignment
policy, affected slots, values before and after, and whether the interpretation
was known. `Observer.Text` receives effective state at each text boundary.
`StateView.Value(slot)` and `StateView.Source(slot)` return the active value and
the proven source-tag index. A negative index denotes a Style value, a default,
or an unprovable source. Views and slices are borrowed for the callback.
`SkipNoEffectProofs` reads the state stream without allocating no-effect
provenance and liveness results.

## Known state and uncertainty

Match status, verified argument shapes, and modeled state are separate:

- A verified, applicable signature permits semantic interpretation; it does
  not by itself establish known state or edit equivalence.
- A source-verified rejected form (the exhaustive `pos` and `move` shapes)
  is ignored by that renderer and cannot acquire a first-wins latch.
  Malformed input still blocks automatic edits; rejection does not prove edit safety.
- A build-conditional or unresolved shape invalidates affected values and
  proofs. When it could have claimed a first-wins slot, a subsequent valid
  assignment cannot fabricate a definite owner.
- A definite first-wins owner survives later conditional operations that
  could not displace it.
- Recognized but unmodeled extensions are barriers rather than invented
  state transitions. Name-unknown or explicitly ignored commands remain
  distinguishable in TagEvent resolution metadata.
- VSFilterMod `\z`, `\frs`, `\fsvp`, `\fshp`, and `\ortho` have modeled
  state for verified signatures.
  Bare forms restore the original Dialogue Style, not the active named reset Style.
  `\frs` truncates its value after multiplying by 10.
  The spacing tags multiply by 8. Depth `\z` multiplies by 80.
  These fields default to zero in each resolved Style. Caller maps stay unchanged.
  A missing Style does not supply known defaults.
  Surplus scalar arguments can reach the source handler, but their shapes remain unresolved here.
- Lua is not modeled. The pinned source compiles Lua code but loads no script
  ([build capabilities](profiles.md#vsfiltermod-build-capabilities)).
- Exact Mod `\fad` has no normalization branch, but its apply branch remains reachable.
  The unrecognized-tag counter does not stop dispatch (RTS.cpp:2668-2675,3055).
- Malformed clip shapes invalidate both rectangle and vector clipping
  values and provenance, including after resets. They never establish
  a *definite* vector owner.
- Transform uncertainty and rendering-time variation are represented
  separately. NoEffect proof revocation is retroactive.

For example, with `{\pos(1,2,3)\pos(4,5)}`, libass and xy-VSFilter
reject the first form and can apply the second. The pinned VSFilterMod build
can apply the first and ignore the second, so the two profiles disagree about
the position owner.

## Current VSFilterMod scope

VSFilterMod handlers use the same concrete/profile engine.
The profile resolver selects Mod assignment metadata.
Document consumers select target-specific Styles before evaluation adds Mod defaults.
The default CLI uses libass validity and xy-VSFilter compatibility, not Mod semantics.

The [tag matrix](../reference/ass-tags.xml) owns signatures and pinned source citations.
`move` accepts exactly four or six non-empty arguments in all three targets.
Eight arguments are rejected before the shared position latch can change.

Mod `pos(x,y,z)` writes depth only when the shared EF_MOVE latch succeeds.
The write is not animated, including inside `t`. A blocked command writes nothing.
Depth has its own provenance. Style resets clear depth but preserve the position latch.
The pinned handler uses a defective Effect layout (RTS.cpp:3530-3539,4238-4241).
Position stays unknown. The command blocks equivalence proofs even when depth is known.

Mod recursively dispatches handlers inside transforms. Effect commands can claim their
first-wins latches. Animated scalar values remain unknown; no static frame value is claimed.
Transforms remain conservative proof barriers in the new Mod scope.
`frs` truncates after interpolation. Equal truncated destinations do not prove equal intermediate frames.
Bare scalar resets and discrete ortho changes also remain conservative inside transforms.

Complex movement, random effects, gradients, images, distortion, and jitter remain barriers.
Complex movers can claim EF_MOVE, so later pos commands cannot invent latch ownership.
`blend` remains unmodeled because its mixer paths differ. Traditional targets still match its `b` prefix.
`xblur` and `yblur` remain barriers until their overlap with both `blur` axis fields is modeled.
No Mod binary or pixel comparison was run. Recheck if pins, build flags,
input signatures, or the modeled renderer fields change.

## Current consumers

All default and explicit-profile consumers use `EvaluateResolved`.
The primary renderer determines invocation validity.
Compatibility comparison and SafeFix checks use the normalized requested profile set.
Consumers can request renderer-local observations:

- `lint.AnalyzeNoEffectsForRenderer` for ASS006 observations;
- `lint.AnalyzeRedundantStyleOverridesForRenderer` for ASS013;
- `fontcheck.ExtractUsages` for font state, uncertainty, source spans, and drawing visibility.

These observations carry no automatic edits. ASS006 owns tag-level no-effect
diagnostics. ASS013 owns the grouping and edit policy for Style-equivalent
runs. Both read this evaluator. Plain Dialogue lines bypass it.

See [lint](lint.md)
for validity diagnostics, [compatibility](compatibility.md) for renderer
comparison, and [contracts](../rules/verification.md) for SafeFix proof scope.
An unmodeled extension remains a proof barrier.

The CLI exposes one `--renderer` list. The first name is primary; the rest extend comparison and proof scope.
JSON and both text presenters name the renderer.
See [usage](../../README.md) and [reports](../usage/diagnostic-reports.md).

## Representation and cost boundaries

Representation changed. Modeled semantics did not. Every consumer still
calls `EvaluateResolved`. Validity, uncertainty, and proof barriers behave as
the sections above describe.

- Slot state is indexed by the modeled slot table, with maps as the fallback
  for a name outside it. A name beyond the table stays correct and only
  loses the index. The latches keep their maps, because the engine sets them
  only for spec-declared first-wins slots, which the table lists today.
- Comparison decides on compact state and materializes the wire audit trace
  at the serialization boundary. A rejected comparison carries no steps.
  The test suite keeps a full-trace oracle and runs both paths on every
  `go test`, including the asymmetric missing-owner case.
- One prepared view per dialogue serves both lint validity and the document
  compatibility pass, so one request resolves each dialogue once.

Interpretation results and `FixProof` bytes are unchanged. A proof carries the
before and after traces as content, so a later read compares them directly.
Cost and benefit measurements are in [performance](../development/performance.md).

## Validation

Use the checked-in contract golden files; do not regenerate a mismatch
without independent source evidence.

```sh
go test ./... -count=1
go test -race ./internal/semantic ./internal/lint -count=1
go vet ./...
go build -o dist/assx ./cmd/assx
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -count=1
go test ./internal/semantic -run '^TestRendererScoped' -count=1
go test ./internal/lint -run '^TestRendererScoped' -count=1
go test ./internal/semantic -run '^$' -bench '^BenchmarkEvaluate(Resolved|Dialogue)' -benchmem
```

The tests cover first-wins divergence,
prefix shadowing, nested transform resolution, clip provenance, style
resets, font consumers, and the absence of single-profile SafeFix edits.

