# Renderer-scoped lint validation (P06/10)

Implementation: [Issue #21](https://github.com/Nemuboshi/assx/issues/21) of
[Epic #16](https://github.com/Nemuboshi/assx/issues/16).

## API and rollout boundary

`lint.AnalyzeForRenderer(dialogue, profile)` and
`lint.AnalyzeDocumentForRenderer(document, profile)` evaluate a **single pinned
renderer**. The document entry point also checks headers, event fields,
Style definitions/references, drawing syntax, and karaoke timing. Existing
`Analyze` and `AnalyzeDocument` retain the frozen default libass/xy-oriented
contract; changing CLI renderer selection and removing the historical input
adapter are reserved for P10.

Only the renderer-neutral concrete syntax tree is used for naming and
argument resolution in the opt-in lint path. The selected `renderer.Profile`
owns prefix collisions, command identity, signature evidence, and conditional
build availability. The shared typed decoder takes the resulting explicit
policy. Lint does not mutate the registry or widen a universal `TagSpec.Counts`
list to legalize VSFilterMod-only shapes.

## Diagnostic semantics

- A source-verified rejected signature is **ASS001**, with valid forms
  sourced from the selected profile. The traditional structural checks and
  pinned rectangle/vector clipping grammar remain strict where evidence is
  not exhaustive.
- Malformed values are **ASS002**. Ignored scalar suffix data stays **ASS019**,
  and raw junk, braces, repeated slashes, and drawing errors retain their
  previous rule identifiers.
- Unknown names are **ASS007**. Names shadowed by prefix dispatch are checked
  as the **actual matched command**: `\\fsvp6` is an invalid `\\fs` argument
  in the traditional profiles, while it resolves to `\\fsvp` in enabled
  VSFilterMod builds.
- **ASS031** is reserved for genuinely new per-profile uncertainty: disabled
  or build-conditional dispatch, a matched but unmodeled command, inferred
  signatures, and malformed/unresolved invocation shapes. It is not a
  cross-renderer incompatibility finding.
- VSFilterMod's verified three-argument `\\pos` is allowed only in a build
  whose extension is enabled. Unknown build capabilities remain unresolved.
  First-wins effects and reset/style provenance are produced by the P05
  semantic evaluator, with no alternate lint state engine.

The optional `renderer` JSON field is present only in explicit per-profile
findings; the default JSON shape, rule IDs, severities, ordering policy, and
exit-code policy remain unchanged. A single-profile analysis **never supplies
SafeFix edits or an edit safety classification**, including on shared
document checks and the existing ASS006/ASS013/font APIs. All-target
equivalence and compatibility are P08 and P07 respectively.

## Regression and performance

`renderer_validation_test.go` exercises independently resolved positions,
prefix collisions, unknown build flags, nested transforms, malformed clips,
first-wins outcomes, Style references, karaoke, drawing, source locations,
deterministic ordering, JSON compatibility, and the single-profile SafeFix
barrier. The default CLI golden contracts remain authoritative; do not update
them without independently verified renderer evidence.

Run `go test ./... -count=1`, the pinned CLI contract test,
`go test -race ./internal/lint ./internal/semantic`, `go vet ./...`, and
`go build ./cmd/assx`. Compare
`go test ./internal/lint -run '^$' -bench '^BenchmarkAnalyzeDocumentRenderer$' -benchmem`
on the same environment, noting that scoped resolution intentionally
requires additional source and signature processing.
