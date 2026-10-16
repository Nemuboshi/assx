# Renderer-scoped lint validation

Implementation: [Issue #21](https://github.com/Nemuboshi/assx/issues/21) of
[Epic #16](https://github.com/Nemuboshi/assx/issues/16).

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
- **ASS031** covers two observation classes. Renderer-scoped validation uses it
  for per-profile uncertainty: disabled or build-conditional dispatch, a
  matched but unmodeled command, inferred signatures, and malformed/unresolved
  invocation shapes. The compatibility APIs use it for unresolved or ignored
  cross-renderer observations, including tracked-state agreement that the
  shared proof gate cannot verify. Verified
  equal tracked state reports `equivalent` and emits no finding. Neither class
  asserts a proven incompatibility; **ASS003** reports those. Style field
  conversion evidence is recorded separately in
  [the Style field matrix](ass-style-fields.xml). That matrix names each
  conversion; it does not by itself resolve the observation.
- Style value checks are separate from conversion evidence. **ASS011** reports a
  fractional integer field, **ASS012** reports a float-precision difference, and
  **ASS022** reports a malformed colour. **ASS032** reports an ASS v4+
  `Alignment` value outside 1 to 9. ASS032 checks a valid value only; it does not
  compare renderer byte order or alignment equivalence. Legacy SSA `Alignment`
  uses a bitfield mapping and is not checked by ASS032.
- **ASS033** reports verified BorderStyle 3 box-scaling differences.
  **ASS034** reports the BorderStyle 4 mode difference.
  They compare declared ASS v4+ Styles only when the relevant resolved fields agree.
  Their conditions and limits are in [the rule catalog](rules.md).
  Both use separate IDs, physical field spans, and selected renderer pairs.
  Neither warning supplies edits or claims final pixel differences.
- VSFilterMod's verified three-argument `\\pos` is allowed only in a build
  whose extension is enabled. Unknown build capabilities remain unresolved.
  First-wins effects and reset/style provenance are produced by the P05
  semantic evaluator, with no alternate lint state engine.
- Renderer-scoped **ASS030** reads the P05 `karaoke_cursor` state from the
  same semantic observer used for other dialogue findings. It also requires
  source-verified signatures for every contributing karaoke tag. Partially
  consumed arguments (such as `\\k200junk`) and inferred-only VSFilterMod
  signatures cannot produce a definitive timing warning. Standalone event
  analysis uses the same evaluator and proof checks, including the default CLI.

The `renderer` JSON field identifies the pinned profile, including Mod capabilities.
Compatibility findings identify both profiles and use physical-line columns.
Primary tag findings use dialogue-relative columns. Findings retain stable source ordering.
Primary error diagnostics produce exit status 1; compatibility diagnostics remain warnings.
ASS005 remains a portability lint for Mod-only names when the requested scope excludes Mod. A single-profile analysis **never supplies
SafeFix edits or an edit safety classification**, including on shared
document checks and the existing ASS006/ASS013/font APIs. Target-configured analysis supplies fixes only after proving every requested profile.
Application recomputes authorized edits with that same primary and compatibility policy. Differential compatibility is provided separately by the
[P07 comparison APIs](renderer-compatibility.md). Those APIs use ASS003 for
proven tag and layout differences, ASS033/ASS034 for the specific Style
differences, and ASS031 for unresolved compatibility observations;
invocation validity remains the responsibility of the APIs described here.

## Implementation record: P06

P06 added observation-only APIs while preserving the temporary default adapter.
P10 removed that adapter and added CLI selection and target-configured fix application.
The reviewed corrections are recorded in [contracts](renderer-contracts.md).

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
