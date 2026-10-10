# Shared dialogue semantic evaluation

Issue: [#4](https://github.com/Nemuboshi/assx/issues/4)

`internal/semantic.Evaluate` is the single state-transition entry point for
the ASS006 no-effect rule, the ASS013 Style-default rule and font analysis.
Future formatters can consume the same state view without depending on lint.

## Declarative special-tag dispatch (Issue #5, part 1)

TagSpec owns the static SemanticKind dispatch policy. Generic assignments
continue to use declared value kinds, affected slots and precedence rules;
a small set of special forms (transforms, clipping, Style resets, karaoke,
drawing mode and relative font sizes) are delegated to
internal/semantic/handlers.go. TransformComparable declares the existing
set of transform targets eligible for no-effect comparison without a
separate tag-name whitelist. This change is a behavior-preserving relocation;
unknown-state handling, observer output and SafeFix policy remain unchanged.

## Inputs and outputs

A caller supplies a lossless `ass.DialogueText`, optional canonical Style
states, the Dialogue Style name and optional observer callbacks. The engine
walks the existing parsed AST, preserving the source tag and source-byte
locations. It does not create a parallel AST or mutate the source.

`Observer.Tag` receives `TagEvent` after each tag's transition. It exposes
the assignment policy, affected slots, values before and after the operation,
and whether the interpretation was known. `Observer.Text` receives the
effective state at each text boundary. `StateView.Value(slot)` and
`StateView.Source(slot)` query the active value and the proven source-tag
index. A negative source index denotes a Style/default value or an
unprovable source. Views and slices are borrowed for the callback; consumers
must copy values that they intend to retain.

Style defaults are resolved lazily rather than copied into every Dialogue.
An explicit `\\r` exposes the original Dialogue Style, and `\\rName`
exposes the named Style when it can be resolved unambiguously. Properties
retained by the renderer across Style resets, including drawing mode and the
karaoke timeline, are preserved. First-wins slots remain latched.

Accumulating karaoke operations expose `karaoke_cursor` in milliseconds;
`\\kt`, malformed durations and uncertain transforms propagate unknown
state. Font analysis reads the effective family, bold, italic and drawing
mode values from the shared text-boundary view.

## Equivalence and SafeFix policy

The lexer/typed decoder may report a syntactically usable numeric prefix,
but canonical state values require fully consumed, renderer-compatible
arguments. `CanonicalTagState` reuses the decoded argument rather than
decoding it twice. Style-relative arguments are resolved using the active
and original Styles, including disagreements on named-reset fallback values.

An unknown or renderer-dependent assignment cannot establish a proven
no-effect source, and an uncertain first-wins owner cannot justify deleting
subsequent tags. When a transform or unknown operation may affect state,
the engine conservatively invalidates the corresponding proof information.
Consumers must not infer SafeFix eligibility from an unknown value.

ASS006 owns tag-level no-effect diagnostics. ASS013 owns the grouping and
editing policy for Style-equivalent runs. Both use the same evaluator in
`AnalyzeDocument`. The existing duplicate-diagnostic suppression still
assigns overlapping fixes to ASS013. A VSFilterMod-only operation, including
one inside a transform, revokes all automatic fixes for that Dialogue.

`SkipNoEffectProofs` permits consumers such as font analysis to use the
state stream without allocating no-effect provenance and liveness results.
Plain Dialogue lines bypass semantic analysis in the document lint pipeline.

## Explicit uncertainty and proof safety (Issue #5, part 2)

TagEvent.Uncertainty reports malformed input, unsupported semantics,
unresolved values, renderer-dependent interpretations and time-dependent
transforms as distinct cases. UncertaintyNone means no ambiguity was
reported for this event; TagEvent.Known still describes canonical assignment
values, not every special operation. Time-dependent transforms invalidate
effective state without automatically becoming global SafeFix barriers.

Actual proof barriers are marked by TagEvent.Barrier and flow through a
shared proof-revocation path. Unresolved named Style resets, unsupported
absolute karaoke timing and invalid karaoke durations now revoke prior
no-effect proofs and block later automated edits. Numeric prefixes with
trailing junk are no longer accepted as canonical karaoke durations.
Known valid tags retain their existing proof and edit behavior.

## Testing and measurement

`internal/semantic/evaluator_test.go` checks state snapshots, source
provenance, Style resets, first-wins, accumulation, unsupported values,
renderer ambiguity and proof-free font reads. Existing ASS006/ASS013, font,
renderer-regression and SafeFix test suites are retained.

Use the benchmark suite in [performance.md](performance.md), measuring
`BenchmarkEvaluateDialogue`, `BenchmarkAnalyzeDocumentFixtures` and
`BenchmarkParseAndAnalyzeDocumentFixtures` before and after refactors.
Evaluate wall time, B/op and allocs/op together. The engine should not
introduce a full snapshot allocation for each individual tag.

## Issue #4 benchmark comparison

Measured in the same Windows/amd64 benchmark environment, using
Go's `-benchmem -benchtime=250ms -count=3`. Baseline was a temporary,
unmodified worktree at `d8006e2` (PR #10 merged). Each number below is
the median of three runs; CI noise and normal host variation still apply.

| Lint workload | Baseline ns/op | Issue #4 ns/op | Baseline B/op | Issue #4 B/op | Baseline allocs/op | Issue #4 allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Typesetting (48 dialogues) | 831,430 | 837,723 | 367,542 | 336,564 | 3,450 | 3,366 |
| Long (10,000 dialogues) | 92,586,333 | 95,386,933 | 38,857,970 | 36,163,037 | 364,722 | 343,762 |

Observed wall-time changes are approximately +0.8% and +3.0%,
respectively, while memory use decreases by approximately 8.4% and
6.9%. The longer workload sees approximately 5.7% fewer allocations.
The comparison measures the complete document lint pipeline, including
ASS006 and ASS013, without document parsing.

## Retroactive SafeFix proof revocation

An unrecognized tag, VSFilterMod-only extension, malformed or
renderer-ambiguous assignment, unsupported transform child, or malformed
structured arity is a dialogue-wide SafeFix proof barrier. Once encountered,
`semantic.Evaluate` marks all previously collected no-effect candidates as
`ProofRevoked` and stops producing new proofs. ASS006 retains those earlier
findings as non-auto-fixable diagnostics with no edits. The ASS013 collector
instead discards its accumulated Style-equivalence edit candidates, even when
the barrier appears inside a transform or after ASS013 already stopped
collecting a run. Known and fully modeled tags retain normal behavior.

Dedicated regressions in `internal/semantic/safefix_barriers_test.go` and
`internal/lint/safefix_barriers_test.go` cover the reported example
`{\fs20\fs20\mystery}A`, extension tags, nested transforms, malformed
arguments, and later text boundaries. This is intentionally conservative:
there may be safe fixes in some specific renderers that are left for manual
review when cross-renderer behavior is uncertain.
