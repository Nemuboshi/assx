# Renderer evidence and non-regression contract (P01)

Epic [#16](https://github.com/Nemuboshi/assx/issues/16), implementation [#17](https://github.com/Nemuboshi/assx/issues/17).

## What is frozen

**Production behavior baseline:** `main@6ba745336615f102d5de39f76550f0a45da7bbb6`.

The six cases in `testdata/contracts/cases/` and their checked-in
`testdata/contracts/golden/*.json` observations form a **behavioral
regression baseline for assx at that commit**. They are not a normative ASS
specification and do not imply that assx currently matches all three
renderers. A future renderer-aware implementation must preserve the default
libass/xy-VSFilter acceptance envelope unless a documented, independently
verified correctness fix deliberately changes it.

`TestRendererBaselineContracts` compares exact JSON snapshots containing:

- Raw input SHA-256, decoded source and byte-exact encode/decode round-trip,
  newline convention, per-dialogue event fields and their source spans.
- Renderer-neutral syntax node types, raw source slices, override-block items,
  tag names, arguments, raw tags and nested transform children with offsets.
- Effective-state tag events (policies, before/after slots, applied/known,
  barrier and uncertainty), state values and provenance indices at each tag
  and text boundary, and no-effect candidates with revoked-proof status.
- CLI JSON diagnostics in their original order, including IDs, severity,
  locations, messages, source citations, SafeFix/UnsafeFix metadata and edit
  spans. The temporary input path is normalized to the checked-in case name.
- Exit code, summary on stderr, file mutation and resulting file SHA-256 for
  default `--format json`, `--fix` and `--unsafe-fix` executions.
- `--check-fonts` diagnostics against the hermetic Go Regular TTF, obtained
  from the existing `golang.org/x/image/font/gofont/goregular` dependency,
  instead of host-installed fonts.

No golden contains elapsed wall-clock time or machine-local temporary paths.
Snapshots are compared byte-for-byte, so diagnostic ordering and the public
JSON shape are covered as well as their values. Existing behavior-specific
unit tests, fuzz cases and rendercheck remain independent of these snapshots.

| Case | Characterization |
| --- | --- |
| `ordinary.ass` | Default clean dialogue, positioning and alignment |
| `renderer-collisions.ass` | `\\fsvp6`, `\\frs`, `\\blend(add)`, 3-argument `\\pos`, first-wins and an earlier malformed position |
| `semantic-barriers.ass` | Malformed rectangular clip, retroactive proof barriers, unknown tags, nested transforms, karaoke, drawing and unresolved reset |
| `style-fonts.ass` | Style lookup, redundant font overrides, missing fonts, missing dialogue style and named reset |
| `utf8bom.ass` | UTF-8 BOM, CRLF, non-ASCII text and SafeFix |
| `utf16le.ass` | UTF-16LE BOM, multilingual text and byte-preserving SafeFix |

The baseline is intentionally *descriptive*: for example, the existing
`\\pos(1,2,3)` rejection in the default profile is frozen even though a
pinned VSFilterMod build can recognize three coordinates. Historical goldens
also retain current `ASS005` prefix-collision diagnostics, but those IDs
are **not** permanent policy invariants: future renderer-specific prefix
resolution may correctly change them, with source evidence and an explicit
golden update. The independent policy tests cover default arity strictness
and prohibit SafeFix across unresolved collisions. P01 makes no production
parser, lint or fix changes.

## SafeFix proof scope

For each fix marked `safe`, assx records every target renderer ID, pinned
version, build capability, source edit range, source hash, edited hash, and
interpretation trace hash. It stores one full proof for an edit group and links
each remaining diagnostic with `fix_proof_ref`. It also stores the
interpretations next to the edits, including dispatch, signature citation,
state, and source provenance. The proof ID hashes every proof field, including
that audit metadata. `--fix` checks the submitted ID and edited-source hash,
then rebuilds the evidence from the current source and complete edit group.

`--fix` rechecks the proof against the current file and the selected edits. The
default scope contains pinned libass and xy-VSFilter profiles. A proof for
VSFilterMod alone cannot authorize a default fix. The default CLI does not
select VSFilterMod.

Use the libass `rendercheck` fixture to compare exact frames after safe fixes.
Use pinned source evidence and independent renderer trace tests for
xy-VSFilter. Do not treat assx diagnostics as an independent renderer oracle.
The CI rendercheck job does not execute xy-VSFilter or VSFilterMod binaries.
Keep a fix unavailable when a selected renderer, signature, build capability,
or semantic operation is unresolved. Keep `\\t()` unavailable because the pinned
matrix verifies only one-to-four-argument transform forms and records layout
changes for the empty form. Keep parenthesized empty `\\b` and `\\i` forms
unavailable until their signatures are verified for that exact form. Keep an
integer Style edit unavailable when the semantic Style model cannot establish
the same modeled value before and after the edit.

The proof compares each renderer's interpretation before and after the edit.
It does not claim cross-renderer pixel equality. It does not prove behavior for
unlisted renderer versions, builds, Lua extensions, or forks.

## Pinned source revisions and build scope

The authoritative machine-readable source pins are
`docs/ass-tags.xml` `<meta><pin>` entries:

| Source | Commit |
| --- | --- |
| libass | `f61db567e6593df3470e91594bcd4ad2d0473aff` |
| xy-VSFilter | `135a30153a38fa846cb5c39df0f258403e92096e` |
| VSFilterMod | `7a00567e4a49b6310691b9a6791646b2a018bfa2` |

The optional local checkouts under `ref/renderers/` are not build artifacts.
The existing `render-equivalence` CI job compiles **libass only**, from
its pinned commit, with the Meson options documented in
`.github/workflows/ci.yml`. It does not experimentally validate Windows
xy-VSFilter or VSFilterMod.

At the pinned VSFilterMod revision, `src/subtitles/RTS.cpp` contains
`#ifdef _VSMOD`-gated extension handlers and additional
`defined(_VSMOD) && defined(_LUA)`-gated Lua paths. The pinned
`src/subtitles/subtitles.vcxproj:95` includes **both** `_VSMOD` and
`_LUA` in one Release configuration. That is evidence for a *build
configuration*, not a guarantee about all binary distributions, modes,
forks, enabled Lua backends or render-time feature availability. Future
renderer profiles must make optional features explicit and must not assume
these capabilities when build configuration is unknown. The tests inspect
the local source when available and otherwise test metadata without
requiring a repository checkout under `ref/`.

## How matrix verification works

Each `<params>` or `<scen>` row with `status="V"` has a
`verified="libass xy vsm"` **subset** of renderer-specific evidence
tokens. This is intentionally separate from the row's status.

Each individual `<sig>` also declares **explicit** `renderer` applicability
(e.g. `renderer="vsm"` for three-coordinate `\\pos`), its own `status`,
`verified`, `inferred`, and exact `cite`. Applicability must equal the
disjoint union of verified and inferred scopes; omission is invalid and can
never mean all renderers. Citations and verification are per signature, and
the parent's verified range is checked against the union of its children.
An inferred renderer has no per-signature proof and cannot silently expand
an arity covered by verified source at the parent level. The negative
mutation suite rejects removing the VSFilterMod-only three-coordinate
restriction or broadening it with unsupported/inferred applicability.

The four evidence classifications are:

- **Verified source:** the renderer is explicitly named by
  `verified`, the row has `status="V"`, and a matching pinned-source
  citation is present. Verified source interpretation still requires
  scenario-specific semantic and edit-equivalence proof before SafeFix.
- **Source-inferred or assumed:** an asserted result without its renderer
  in `verified`, or a row with `status="S"`. It may inform investigation
  but must never establish renderer equivalence.
- **Unsupported:** a renderer *result* of `unsupported`. This only
  becomes a verified assertion when that exact renderer cell is also
  source-verified; an unverified unsupported claim cannot justify removing
  an operation.
- **Unchecked:** explicit `unchecked`, or an absent renderer result or
  scenario row. Missing entries and inherited scenario defaults are
  unverified, even if another renderer's result is verified.

These are orthogonal dimensions: syntactic acceptance, effective state,
cross-renderer compatibility, and safe-to-edit equivalence must not be
collapsed into a single 'valid tag' or 'safe tag' flag.

`TestMatrixVerifiedCellScope` recomputes each verified scope from the
existing row status, explicit renderer result and matching source citation.
The per-signature validator additionally rejects missing applicability,
unjustified scope expansion, invalid child citations and mismatched group
evidence. `TestMatrixSignatureApplicabilityMutations` exercises XML mutations
that would otherwise broaden VSFilterMod-only arities.
Together they reject duplicate, invented, ungrounded or incorrectly ordered
scopes.
`TestMatrixRendererSourcePins` checks the exact full commits and ensures
the libass rendercheck pin agrees. The existing XML tests still check
vocabulary, signatures, statuses, citations and tag registry membership.
If a newly investigated source changes evidence, update the single XML
matrix and its own source-local assertions, rather than duplicating a
renderer-tag table in the test harness.

## Reproduce

From the repository root:

```sh
go test ./...
go vet ./...
go build ./cmd/assx
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -count=1 -v
go test ./internal/ass/spec -run '^TestMatrix' -count=1 -v
```

For the **deliberate** baseline-refresh operation only:

```sh
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -update-contracts -count=1
git diff -- testdata/contracts/golden
```

Do not run `-update-contracts` as an automatic response to failing tests.
In particular, a golden mismatch is evidence that needs review, not a
request to regenerate the expectation. Golden updates are committed with
the behavior change and narrowly justified by a pinned-source citation.

## Intentional corrections and PR review

When a later PR must correct previously accepted behavior:

1. Identify the affected renderer(s), precise source revision and cited
   source code. Distinguish static source inference from a reproducing
   executable oracle; record the build flags needed for a runtime claim.
2. Write a targeted, independent regression that demonstrates the incorrect
   behavior and the corrected interpretation. Include a negative test for
   default strictness and SafeFix where relevant.
3. Show the before/after golden diff, with exact changed IDs, severities,
   line/column positions, ordering, exit codes, JSON keys, state and source
   provenance, as well as fix metadata and bytes. Explain every intentional
   change explicitly in the PR.
4. Only then regenerate and review the affected golden file(s). Never hide
   unrelated differences behind a broad snapshot rewrite. No renderer
   receives SafeFix without an equivalence proof under that specific
   requested renderer and build capability scope.
5. Compare benchmark samples using the *same* Go toolchain, CPU and
   workloads; benchmark noise is not a correctness failure.

P08 adds renderer-scoped proofs to `SafeFix` diagnostics and rechecks each
selected edit before `ApplyFixes` changes the source. Keep default proofs scoped
to pinned libass and xy-VSFilter. Keep renderer-local analysis free of `SafeFix`
metadata unless it proves every selected target. Review changed snapshots for
proof scope, fix availability, output bytes, and summary text before updating
the goldens.

## Performance baseline

The raw, benchstat-compatible input is
`docs/performance/baseline-6ba7453-windows-amd64.txt` (source code at
`6ba7453`, plus P01 tests/documentation only). The run used
Windows/amd64, Go 1.27.0,
`-benchmem -benchtime=250ms -count=5`, without `-race`.
The fixture-driven parse, AST, state, lint-only and parse-plus-lint
benchmarks include both short files and the synthetic 10,000-dialogue
workload. See [performance.md](performance.md) for timing boundaries and
benchstat usage. The recorded samples are a **comparison input**, not a
cross-machine absolute performance budget.
