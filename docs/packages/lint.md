# Renderer-scoped lint validation

The run order and the proof boundary are in [the lint pipeline](../architecture/lint-pipeline.md).

## Current API boundary

`lint.AnalyzeForRenderer(dialogue, profile)` and
`lint.AnalyzeDocumentForRenderer(document, profile)` evaluate a **single pinned
renderer**. The document entry point also checks headers, event fields,
Style definitions/references, drawing syntax, and karaoke timing.
These observation-only APIs do not authorize edits.
`AnalyzeDocumentForTargets` accepts a primary profile and compatibility profiles.
VSFilter primaries project fixed ASS v4+ record slots, including when Format fields differ.
Style and Dialogue rows form separate groups.
A group projects only when every row has a known ASS v4+ layout.
Unknown groups keep the parser's original fields. This is not a complete renderer
loader.
`Analyze` and `AnalyzeDocument` call the same concrete/profile pipeline with libass primary and xy-VSFilter compatibility.
There is no historical input adapter or duplicate semantic path.

Only the renderer-neutral concrete syntax tree is used for naming and
argument resolution in every lint path. The selected `renderer.Profile`
owns prefix collisions, command identity, signature evidence, and conditional
build availability. The shared typed decoder takes the resulting explicit
policy. Lint does not mutate the registry or widen a universal `TagSpec.Counts`
list to legalize VSFilterMod-only shapes.

## Diagnostic scope and policy

- A single-profile analysis never supplies SafeFix edits or an edit safety
  classification. This holds for shared document checks and for the ASS006,
  ASS013, and font APIs.
- Target-configured analysis supplies fixes only after it proves every requested
  profile. Application recomputes the authorized edits under the same primary and
  compatibility policy.
- The `renderer` JSON field names the renderer or renderer pair.
- Compatibility findings identify both profiles and use physical-line columns.
  Primary tag findings use dialogue-relative columns. Findings keep stable source
  ordering.
- Primary error diagnostics produce exit status 1. Compatibility diagnostics
  remain warnings.
- First-wins effects and reset/style provenance come from the semantic evaluator.
  Lint keeps no second state engine.

The rule catalog owns each condition, severity, and limit:
[rule catalog](../reference/catalog.md). The profile page owns prefix collisions
and build availability: [profiles](profiles.md). The comparison APIs own
differential results: [compatibility](compatibility.md).

## Regression and performance

`renderer_validation_test.go` exercises independently resolved positions,
prefix collisions, nested transforms, malformed clips,
first-wins outcomes, Style references, karaoke, drawing, source locations,
deterministic ordering, JSON compatibility, and the single-profile SafeFix
barrier. The default CLI golden contracts remain authoritative; do not update
them without independently verified renderer evidence.

Run `go test ./... -count=1`, the pinned CLI contract test,
`go test -race ./internal/lint ./internal/semantic`, `go vet ./...`, and
`go build -o dist/assx ./cmd/assx`. Compare
`go test ./internal/lint -run '^$' -bench '^BenchmarkAnalyzeDocumentRenderer$' -benchmem`
on the same environment, noting that scoped resolution intentionally
requires additional source and signature processing.
