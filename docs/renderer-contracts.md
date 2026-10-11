# Renderer evidence and non-regression contract

Epic [#16](https://github.com/Nemuboshi/assx/issues/16), implementation [#17](https://github.com/Nemuboshi/assx/issues/17).

## Baseline and current contract

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
- Concrete syntax node types, raw source slices, override-block items,
  lexical expressions, heads, commas, parentheses, and exact source offsets.
- Explicit pinned libass state events and text-boundary observations.
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

## Implementation record: P01

The original baseline is intentionally *descriptive*: for example, the existing
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

`--fix` rechecks the proof against the current file and selected edits.
Proof construction starts with the original parsed document. It builds an event
and Style view for each target.
Application rebuilds each view from the current source under the requested
primary and compatibility policy.
It does not trust diagnostic scope labels.
Subset, revision, capability, source, edited hash, or interpretation mismatches reject the request.
Separate proof groups also require combined edit verification before application.
A custom layout cannot borrow the primary renderer's integer field proof. The
default scope contains pinned libass and xy-VSFilter profiles. A proof for
VSFilterMod alone cannot authorize a default fix. The default CLI does not select VSFilterMod.
`--renderer` selects primary validity. `--compat` extends compatibility and proof targets.
An explicit renderer without compatibility selects only that primary.
An explicit empty compatibility value also selects only the primary.
Profiles are normalized by renderer, revision, and build; different builds remain distinct.
Compatibility validity never becomes a primary error.
Mod capability flags default to unknown and require a requested Mod profile.

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

## Implementation record: P08

P08 added renderer-scoped proofs to `SafeFix` diagnostics and rechecks each
selected edit before `ApplyFixes` changes the source. Keep default proofs scoped
to pinned libass and xy-VSFilter. Keep renderer-local analysis free of `SafeFix`
metadata unless it proves every selected target. Review changed snapshots for
proof scope, fix availability, output bytes, and summary text before updating
the goldens.

## P10 reviewed correction table

P10 replaces the legacy syntax snapshots with concrete expressions and explicit libass state.
The original `source_commit` identifies baseline provenance, not the current engine.
No case input, encoding, BOM, newline, or round-trip bytes change.
The table below describes intentional changes before refreshing the six snapshots.

| Correction | IDs, severity, columns, and order | JSON, proofs, bytes, and exit | Independent check and evidence |
| --- | --- | --- | --- |
| Single concrete syntax | Lexical heads replace inferred tag items; state events retain resolved names and source columns. | Items use `expression` and concrete spans instead of `tag`; state uses explicit libass. | Concrete round-trip/nesting tests; syntax is source-preserving, not a renderer claim. |
| Pinned scope | Existing primary rules retain their severity and dialogue-relative locations. | `renderer` includes revision and Mod capabilities; plain/pretty display scope. | Target CLI and report span tests; pinned dispatch sources below. |
| Default compatibility | Adds ASS003 warnings for differing Style layouts and ASS031 warnings for unresolved values, application, ownership, or state. | Pair scope and `source_span` use absolute decoded bytes; `field` identifies dimension. No edits or primary errors. | `TestDefaultCompatibilityAddsScopedObservations`, document layout tests; libass `ass.c:478-529,640-762`, xy `STS.cpp:1425-1558`. |
| Prefix collisions | At renderer-collisions 15:3/9/13 and 17:7, add ASS002 errors for actual `fs`, `fr`, `b`; retain ASS005 portability warnings. | No Mod signature is accepted in default mode; no SafeFix. Existing exit 1 remains 1. | `TestDefaultProfilePrefixCorrections`, independent prefix fence; libass `ass_parse.c:355-916`, xy `RTS.cpp:1716-1769,2155-2163`, Mod `RTS.cpp:2524-2669`. |
| Unknown longer head | semantic-barriers 16:13 remains ASS007 suggestion; resolved tag changes `unknownTag` to `u`. | State reports actual prefix and remains a retroactive fix barrier. Exit 1 remains from malformed clip. | Independent unknown-operation fence; same prefix sources. |
| Rejected position | ASS001 errors and columns remain strict; malformed input still fences fixes. | Rejection cannot own a latch; later valid position runs. Compatibility application can still compare verified rejection. | Rejected-position, arity, and first-wins tests; libass `ass_parse.c:608-622`, xy `RTS.cpp:2615-2628`. |
| Target-specific Style proof | ASS011/ASS013 observations remain, but custom layouts cannot borrow another target's field type or reset defaults. | Recompute every requested profile and combined edit group. Preserve source when a target is unresolved. | Wrong-layout/source-equality and combined-reset/transform tests; libass `ass.c:640-762`, xy `STS.cpp:1497-1558`, reset `RTS.cpp:2644-2648`. |
| Presenter proof metadata | Badge counts follow valid proof identity and grouped references, including JSON round trips. | Empty, tampered, dangling, or conflicting proofs cannot produce SafeFix badges. | `TestReportFixOutcomesUseValidatedGroupedProofs`; proof authorization is an assx contract, not renderer evidence. |
| Encoding fixtures | utf8bom and utf16le keep ASS006 suggestion at 15:3 and optional ASS016 at 15:13. | One proved duplicate removal remains; encoded output hashes and exits stay unchanged. Compatibility warnings remain after fixing. | UTF-8/UTF-16 file tests, proof/source-equality tests; per-target assignment checks. |

| Case | Final diagnostic counts: default / fonts / safe-fix / unsafe-fix | Exits in all modes | Output bytes |
| --- | --- | --- | --- |
| ordinary | 6 / 6 / 6 / 6 | 0 | Identical to prior observations |
| renderer-collisions | 37 / 37 / 37 / 37 | 1 | Identical to prior observations |
| semantic-barriers | 39 / 39 / 39 / 39 | 1 | Identical to prior observations |
| style-fonts | 17 / 18 / 12 / 12 | 0 | Identical to prior observations, including fixed output |
| utf8bom | 7 / 8 / 5 / 5 | 0 | Identical to prior observations, including fixed output |
| utf16le | 7 / 8 / 5 / 5 | 0 | Identical to prior observations, including fixed output |

Ordering is stable by line and column. Primary findings precede portability at equal locations.
Compatibility dimensions retain their semantic order at equal physical locations.
Equal locations do not mean duplicate diagnostics when dimensions differ.

For `ordinary.ass`, the default adds six compatibility warnings; all four modes keep exit 0 and identical file bytes.
For `renderer-collisions.ass`, all modes keep exit 1, no fixes, and identical file bytes.
For `semantic-barriers.ass`, all modes keep exit 1, no fixes, and identical file bytes.
For `style-fonts.ass`, primary ASS013/ASS006 fix groups remain proved across both target views.
Both fix modes retain the same output bytes and exit 0; compatibility warnings remain.
For each encoding case, both fix modes remove the same duplicate tag and retain the original encoding.
All unmodified modes retain the input hash. No unsafe edit appears in these cases.
Proof hashes change where target-specific Style context changes the interpretation trace; target pins and edit bytes do not broaden.

Full pinned source links:

- [libass parser](https://github.com/libass/libass/blob/f61db567e6593df3470e91594bcd4ad2d0473aff/libass/ass_parse.c#L355-L916)
- [libass document](https://github.com/libass/libass/blob/f61db567e6593df3470e91594bcd4ad2d0473aff/libass/ass.c#L478-L762)
- [xy dispatch](https://github.com/Masaiki/xy-VSFilter/blob/135a30153a38fa846cb5c39df0f258403e92096e/src/subtitles/RTS.cpp#L2155-L2163)
- [xy document](https://github.com/Masaiki/xy-VSFilter/blob/135a30153a38fa846cb5c39df0f258403e92096e/src/subtitles/STS.cpp#L1425-L1558)
- [Mod dispatch](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2524-L2669)

These claims are source-verified or assx contract tests, not new renderer runtime measurements.
P09 handlers and XML corrections use the P10 concrete/profile engine.
Mod semantics do not broaden the default traditional target set.

### P09/P10 integration: citation-only baseline correction

The `renderer-collisions.ass` golden adds the P09 Mod citation to the verified `move` repeat scenario.
The four CLI modes change only diagnostic 26's first `sources` entry.
The existing libass and xy citations remain. The added citation is `vsm:src/subtitles/RTS.cpp:3441,3510-3515`.
The [matrix](ass-tags.xml) records the shared position latch at these pinned source lines.
See the [Mod handler](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L3439-L3515).
No fixture, semantic snapshot, diagnostic count, output bytes, fix authorization, or exit status changes.
The remaining five goldens stay unchanged from P10.
This evidence update does not add Mod to the default target set.

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
