# Evaluation model

Status: Phase 0 to Phase 4 are implemented. Current behavior is in
[the lint pipeline](lint-pipeline.md) and
[compatibility](../packages/compatibility.md).

## Purpose

The evaluator gives each selected renderer one state answer. Two limits block
that goal:

1. Tag value rules are global. `internal/ass/spec/tags.go` holds one `Min`,
   `Max`, `Value`, `Slots`, and `Behavior` per tag name. Two renderers cannot
   disagree about a legal range or a value kind.
2. Renderer differences live in code. `internal/semantic/handlers.go` and
   `internal/semantic/effective.go` branch on `profile.Kind()`. Nested
   transform children use recursion, so each renderer walks the same tree
   again.

The design fixes both limits. Tag specifications become per renderer. The
nested dialogue becomes one linear program. Evaluation folds that program with
one state per selected renderer.

## Part 1: per-renderer tag specification

### Goal

One lookup answers: for this renderer, this command, this arity, and this
argument, what value kind and range apply?

### Evidence schema

The evidence matrix already scopes signatures per renderer with
`sig@renderer`. Extend the argument rows with structured fields. Keep the
prose `range` for documentation.

```xml
<sig n="1" form="both" renderer="libass xy vsm" status="V" ...>
  <p i="1" name="style" kind="int" min="0" max="3" renderer="libass xy vsm"
     range="0..3; outside selects the script WrapStyle"/>
</sig>
```

Rules:

- `kind` maps to `spec.ValueKind`: `int`, `number`, `bold`, `hex`, `font`,
  `coord`, `drawing`, `none`.
- `min` and `max` are optional. They apply only to `int` and `number`.
- `p@renderer` is optional. Without it, the row applies to all renderers that
  the parent signature covers.
- Rows for one argument index and one arity must not overlap. The generator
  rejects an overlap.
- Prose `range` never feeds code. Only structured fields do.

A tag-level block carries the semantic fields that are still in Go:

```xml
<spec renderer="vsm" n="1" form="bare"
      slots="alignment" behavior="first-wins" semantic="alignment"
      transform-children="recursive"/>
```

`<spec>` is optional. A missing block falls back to `spec.TagSpecs`. A missing
renderer row falls back to the tag default. `n` and `form` are optional and
default to all arities.

### Generated form

`internal/ass/renderer/matrix_generated.go` gains:

```go
type generatedArgSpec struct {
    scope    uint8 // renderer mask
    count    int
    form     Form
    kind     spec.ValueKind
    min, max int
    hasRange bool
}
type generatedTagSpec struct {
    scope uint8
    count int
    form  Form
    slots []string
    behavior spec.Behavior
    semantic spec.SemanticKind
    transformChildren TransformChildrenMode
}
```

Selection order for a profile:

1. A matching `generatedArgSpec` or `generatedTagSpec` row.
2. `spec.VSFilterModTagSpecs`, when the profile is VSFilterMod.
3. `spec.TagSpecs`.

The profile selects the row. `renderer.Result.Policy` stays one
`spec.TagSpec`, because one result already belongs to one profile.

### Dispatch integration

`internal/ass/renderer/dispatch.go` sets `Result.Policy` from
`spec.TagSpecs[name]`. It must instead call one selector:

```go
func (p Profile) CommandSpec(name string, count int, form Form) (spec.TagSpec, bool)
```

The selector merges the tag-level fields and the argument-level value fields.
It returns the same `spec.TagSpec` shape that `validateIR` and the evaluator
use today.

### Validation

- `go generate ./internal/ass/renderer` emits the new tables. `-check` fails
  on stale output.
- A parity test asserts that every tag without a matrix override selects the
  same spec as today. A migration mistake then fails a test, not a render.
- A test asserts that no two argument rows for one tag, arity, and argument
  index overlap.
- `TestMatrixRendererSourcePins` and `matrixSHA256` stay unchanged in purpose.

## Part 2: linear program

### Goal

Build the dialogue once. Remove recursion from evaluation. Do not parse or
resolve again for a second renderer.

### Events

```go
type programEvent struct {
    kind  eventKind // text, tag, scopeOpen, scopeClose
    tag   ass.Tag
    index int       // tag ordinal in emission order
    text  string
    start int
    scope int       // enclosing scope id, -1 for top level
}

type programScope struct {
    id       int
    parent   int
    tagIndex int // the \t tag that opens this scope
    children []int // event indexes, in source order
}
```

The program is profile-neutral. Arguments and policies are not stored in it.
The fold reads `resolvedOperation` from the profile's prepared dialogue by
`tag.Start`, as `evaluator.operationFor` does today.

### Transform scopes

A `\t(a,b,children)` tag emits `scopeOpen` before its children and
`scopeClose` after them. The evaluator keeps a scope stack of ids, not a
recursive call. Time bounds stay on the scope.

The transform-children mode is a spec field:

- `recursive`: apply each child handler, then invalidate the child slots
  (current VSFilterMod behavior).
- `static`: publish no static value for a child assignment (current libass
  and xy-VSFilter behavior).

`transitionHasNoEffect` reads children through the scope, not through
`tag.Children`.

### Ordering invariant

The program order must equal the current `renderer.Profile.WalkDialogue`
emission order. `PreparedDialogue.EachOperation` must keep that order. The
compact proof trace depends on it.

## Part 3: multi-state fold

### Goal

Walk the program once. Produce one evaluation per selected renderer. Do not
run a second dialogue pass.

### State

```go
type stateSet struct {
    profiles   []renderer.Profile
    stores     []slotStore      // one per profile
    live       []bitset         // one per profile
    candidates []map[int]noEffectCandidate
    proofs     []bool           // one per profile
}
```

A shared store is not correct. The stores stay separate, because each
renderer owns its state. Only the traversal, the scope stack, and the text
walk are shared.

### Output

```go
type EvaluationSet struct {
    ByProfile []Evaluation // parallel to the requested profiles
}
```

Each `Evaluation` keeps its current shape. The new `NoEffects` rows identify
the profile. Lint maps a compared `NoEffect` or karaoke observation to an
`Observation`, as it does for compatibility findings.

### Lint integration

- The primary pass keeps validity, edits, and SafeFix authority.
- Compared evaluations supply observations only. They never supply edits.
- A primary `no-effect` finding keeps the existing overlap suppression. A
  compared observation attaches to that finding. It does not create a second
  suppression pass.
- The compared value check in `internal/lint/observations.go` becomes
  redundant once `Result.Policy` is per profile and the compared value check
  runs from the per-profile spec. Remove it then.

## Branch inventory

Each renderer branch becomes a spec field.

| Site | Rule | State |
| --- | --- | --- |
| `semantic/effective.go` | Mod Style transform | `behavior@style-transform` |
| `semantic/effective.go` | `\pos` with three arguments sets Mod depth | `spec@position-depth` |
| `semantic/effective.go` | Mod applies effect handlers inside transforms | `behavior@effect-in-transform` |
| `semantic/handlers.go` | Mod transform children invalidate all slots | `behavior@transform-children` |
| `semantic/handlers.go` | Mod-only tag with empty arguments resets the Style | tag scope |
| `semantic/handlers.go` | `\ortho` zero/one argument semantics | tag scope |
| `lint/portability.go` | Mod-only tag warns under another primary | unchanged: target policy, not evaluation |
| `semantic/compatibility_styles.go` | Style layout differs per renderer | unchanged: comparison reporting |

## Invariants

These rules must not change.

- The primary renderer decides error severity. A compared rejection is a
  warning unless the primary also rejects.
- Only the primary pass authorizes edits. A compared evaluation never yields
  `Edits`, `FixSafety`, or `FixProof`.
- `compact` proof comparison produces the same trace as the full evaluator.
  `TestCompactCompareMatchesFullReference` must pass at every phase.
- `EachOperation` order is stable.
- The contract goldens change only through the two-key review path.
- The matrix digest changes only when the XML changes.

## Phases

### Phase 0: schema and selector

Implemented.

- `docs/reference/ass-tags.xml` argument rows accept `value`, `min`, `max`,
  and a `renderer` scope.
- `cmd/genmatrix` validates the fields and emits `params` per signature.
- `Profile.CommandSpec` selects the row. `Resolve` publishes it as
  `Result.Policy`.
- A prebuilt override table keeps resolution free of matrix walks.

Exit criteria:

- `go generate` is deterministic and `-check` passes.
- The parity test proves the selector equals the current tables for every tag
  without overrides.
- `go test ./...` passes.
- Performance moves less than 1%.

### Phase 1: per-renderer value fields

Implemented for the tags that carry a concrete value kind. The matrix now
holds `value` for 44 tags, with `min`/`max` for `i`, `u`, `s`, `q`, `an`, and
`a`. Prose `range` stays documentation.

No verified per-renderer range difference exists in the matrix today. The
mechanism is proven with a synthetic matrix row: one profile accepts 0..100
and another accepts 0..60. A value of 61 selects a different policy per
profile, and `validateIR` reads that policy.

Exit criteria:

- `validateIR` reads only `Result.Policy`.
- The new test fails when the compared range is not used.
- No golden change for documents without the new case.

### Phase 2: linear program

Implemented in `internal/semantic/operation.go`.

`buildProgram` returns one neutral `program`: the token stream and a
child-index map. Evaluation reads `children[tag.Start]` instead of recursing
through `tag.Children`. Evaluator tags no longer publish a borrowed child
tree; the contract records the flat output. The compact trace is unchanged.

Exit criteria:

- `TestCompactCompareMatchesFullReference` passes.
- All renderer evaluation tests pass without edits.
- `BenchmarkSessionDiscovery` stays within 2% of the current value
  (`scaling_250` about 27.6ms, `long_10000` about 443ms).

### Phase 3: branch removal

Implemented for the evaluator.

- `<meta><behavior renderer="...">` carries `transform-children`,
  `effect-in-transform`, and `style-transform`.
- `<tag><spec renderer="...">` carries `position-depth` for `pos`.
- The Mod-only empty-argument reset and the `ortho` check no longer test
  `Kind()`, because tag scope already decides them.

`Kind()` remains in comparison reporting, the portability heuristic, and one
lint evidence gate. Those answer different questions and stay.

Exit criteria:

- No `profile.Kind()` branch remains in the evaluator, except where a value
  is genuinely renderer-specific data.
- One regression test per removed branch.
- Performance stays within 2%.

### Phase 4: multi-state fold

Implemented.

`DialoguePreparations` caches one neutral program per dialogue text. Every
profile resolves its own operation map over that program.
`EvaluatePreparedSet` groups the prepared dialogues by program and folds each
group once with one state per prepared dialogue. A custom event Format can
select a different Text span for each renderer, so entries from different text
spans have different programs and never share one fold. The lint compared pass
uses the set for karaoke and drawing on dialogues that use `\k`, `\K`, `\kf`,
`\ko`, `\kt`, or `\p`.

The primary renderer keeps its own pass because it also runs the lint rules and
authorizes fixes. The set covers all compared renderers together.

Compared value checks follow the same rule: an expression whose decode equals
the primary's copies the primary verdict. A different decode runs the tag
cascade once and can create a finding. The per-finding value check is gone.

Exit criteria:

- A compared karaoke or drawing difference appears as an observation on the
  same condition.
- The compared value check from Phase 1 replaces the lint-level deep check.
- `BenchmarkSessionDiscovery` stays within 3% of the 28.7ms and 456ms values
  from Phase 3.
- CLI end-to-end on 2000 dialogues stays within 3% of 552ms.

## Tests

- `TestMatrixRendererSourcePins`: pinned source rows stay intact.
- `TestCommandSpecMatchesTables`: generated selection equals the fallback.
- `TestArgumentSpecRowsDoNotOverlap`: schema validation.
- `TestValueRangeDiffersByRenderer`: the 61 case.
- `TestCompactCompareMatchesFullReference`: trace equivalence.
- `TestRendererBaselineContracts`: contract drift through the review path.
- `TestEvaluatePreparedSetSharesProgram`: one program, two states.
- `TestEvaluatePreparedSetSplitsDifferentTexts`: a program per text, no mixed
  fold.
- `TestSameResolution`: structural equality gates the compared rule cascade.
- `BenchmarkSessionDiscovery`: scaling gate.

## Open questions

- Which tags have verified per-renderer value ranges today? The matrix prose
  suggests none. Phase 1 needs one real case or a synthetic fixture.
- Should `<spec>` be per tag or per arity? `\clip` changes slots by arity, so
  per arity is the safer shape.
- Does the compact trace need a per-profile tag after multi-state evaluation?
  Keep one trace per profile if the proof audit needs it.
