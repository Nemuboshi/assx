# Renderer evidence and non-regression contract

## Baseline and current contract

**Production behavior baseline:** `main@6ba745336615f102d5de39f76550f0a45da7bbb6`.

The six cases in `testdata/contracts/cases/` and their checked-in
`testdata/contracts/golden/*.json` observations form a **behavioral
regression baseline for assx at that commit**. They are not a normative ASS
specification and do not imply that assx currently matches all three
renderers. Preserve the default libass/xy-VSFilter acceptance envelope unless a
documented, independently
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
unlisted renderer versions, builds, Lua scripts, or forks.

## Pinned source revisions and build scope

The authoritative machine-readable source pins are
`docs/ass-rules/ass-tags.xml` `<meta><pin>` entries:

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
`#ifdef _VSMOD`-gated extension handlers and
`defined(_VSMOD) && defined(_LUA)`-gated Lua paths. The pinned project files
define both gates in every configuration. That is evidence for a *build
configuration*, not a guarantee for every binary distribution or fork. assx
models only `_VSMOD`: the gate compiles Lua code, but the pinned source loads
no script. See [build capabilities](profiles.md#vsfiltermod-build-capabilities).
The Mod capability stays unknown until selected. The tests inspect the local source
when available and otherwise test metadata without requiring a repository
checkout under `ref/`.

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
go build -o dist/assx ./cmd/assx
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

## Performance baseline

The raw, benchstat-compatible input is
`docs/development/baseline-6ba7453-windows-amd64.txt`. See
[performance](../development/performance.md) for the environment, sample settings, timing
boundaries, and benchstat usage. The recorded samples are a **comparison
input**, not a cross-machine absolute performance budget.
