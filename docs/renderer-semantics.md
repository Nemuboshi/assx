# Renderer-scoped effective semantics

Implementation: [Issue #20](https://github.com/Nemuboshi/assx/issues/20) in
[Epic #16](https://github.com/Nemuboshi/assx/issues/16). This builds on the
independent syntax and source-backed dispatch delivered in P02–P04.

## Evaluation boundary

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
  state for verified signatures when `_VSMOD` is enabled and Lua is disabled.
  Bare forms restore the original Dialogue Style, not the active named reset Style.
  `\frs` truncates its value after multiplying by 10.
  The spacing tags multiply by 8. Depth `\z` multiplies by 80.
  These fields default to zero in each resolved Style. Caller maps stay unchanged.
  A missing Style does not supply known defaults.
  Surplus scalar arguments can reach the source handler, but their shapes remain unresolved here.
- Lua globals run before built-in dispatch. Unknown or enabled `_LUA`
  capabilities therefore block every tag, including core names.
- Exact Mod `\fad` has no normalization branch, but its apply branch remains reachable.
  The unrecognized-tag counter does not stop dispatch (RTS.cpp:2668-2675,3055).
- Malformed clip shapes invalidate both rectangle and vector clipping
  values and provenance, including after resets. They never establish
  a *definite* vector owner.
- Transform uncertainty and rendering-time variation are represented
  separately. NoEffect proof revocation is retroactive.

For example, with `{\pos(1,2,3)\pos(4,5)}`, libass and xy-VSFilter
reject the first form and can apply the second. VSFilterMod with `_VSMOD`
enabled can apply the first and ignore the second. A build with unknown
capabilities cannot prove which command owns the position slot.

## Current VSFilterMod scope

P09 Mod handlers use the P10 concrete/profile engine.
The profile resolver selects Mod assignment metadata.
Document consumers select target-specific Styles before evaluation adds Mod defaults.
The default CLI uses libass validity and xy-VSFilter compatibility, not Mod semantics.

The [matrix](ass-tags.xml) owns signatures and pinned source citations.
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

Complex movement, random effects, gradients, images, distortion, jitter, and Lua remain barriers.
Complex movers can claim EF_MOVE, so later pos commands cannot invent latch ownership.
`blend` remains unmodeled because its mixer paths differ. Traditional targets still match its `b` prefix.
`xblur` and `yblur` remain barriers until their overlap with both `blur` axis fields is modeled.
No Mod binary or pixel comparison was run. Recheck if pins, build flags,
input signatures, Lua assumptions, or the modeled renderer fields change.

## Current consumers

All default and explicit-profile consumers use `EvaluateResolved`.
The primary renderer determines invocation validity.
Compatibility comparison and SafeFix checks use the normalized requested profile set.
Consumers can request renderer-local observations:

- `lint.AnalyzeNoEffectsForRenderer` for ASS006 observations;
- `lint.AnalyzeRedundantStyleOverridesForRenderer` for ASS013;
- `lint.AnalyzeFontsForRenderer` for font state and drawing visibility.

These observations carry no automatic edits. See [scoped lint](renderer-lint.md)
for validity diagnostics, [compatibility](renderer-compatibility.md) for renderer
comparison, and [contracts](renderer-contracts.md) for SafeFix proof scope.
An unmodeled extension remains a proof barrier.

The CLI exposes `--renderer`, `--compat`, and explicit Mod build capabilities.
JSON and both text presenters identify the pinned scope.
See [usage](../README.md) and [reports](diagnostic-reports.md).

## Implementation record: P05

P05 preserved the then-default adapter while introducing resolved evaluation.
P10 removed that adapter after targeted regressions and reviewed baseline corrections.

## Implementation record: analysis and SafeFix performance

The analysis and SafeFix performance work (#42, #57, #7) changed
representation, not modeled semantics. Every consumer still calls
`EvaluateResolved`, and validity, uncertainty, and proof barriers behave as
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

Interpretation results, proof hashes, and `FixProof` bytes are unchanged.
The after-trace digest cache keys on the edited text and the Dialogue Style,
because a trace covers the interpreted Style: two dialogues that share a text
under different Styles keep separate digests.
Cost and benefit measurements are in [performance](performance.md).

## Implementation record: unused helper removal

At `85b236b`, `semantic.ParseHex`, `SameSlotValues`, and `SetSlotValues` had no callers, including tests.
Go `deadcode` v0.52.0 with `-test ./...` and `gopls references` confirmed this on Windows/amd64 with Go 1.27.0.
The cleanup removes these helpers without changing renderer behavior.
Commit `ff992a7` replaced slot comparison with `evaluator.sameSlotValues`.
Commit `5e58e05` replaced slot assignment with `slotStore.setAssigned`.
This finding is source-verified for the default build at that revision.
Check again if the source or build scope changes.

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
