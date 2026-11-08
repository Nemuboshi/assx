# Verification rules

## Baseline

The production behavior baseline is `main@6ba745336615f102d5de39f76550f0a45da7bbb6`.
`testdata/contracts/cases/` holds the six baseline cases and
`testdata/contracts/golden/*.json` holds their observations. They form a
behavioral regression baseline for assx at that commit. They are not a normative
ASS specification and do not imply that assx matches all three renderers.

Preserve the default libass/xy-VSFilter acceptance envelope unless a documented,
independently verified correctness fix deliberately changes it.

Snapshots are compared byte-for-byte, so diagnostic order and the public JSON
shape are covered as well as their values. No golden contains elapsed wall-clock
time or a machine-local temporary path.

A snapshot records no digest that only repeats content recorded beside it. It
keeps the source digest, because the source text is not in the snapshot.

Do not regenerate a snapshot as a response to a failing test. A mismatch is
evidence that needs review. Regeneration has two keys and one rule:

1. Name the reviewed cause:
   `ASSX_CONTRACT_REVIEW="remove unused proof capabilities"`.
2. Write candidates first: add `-review-contracts`. They land in
   `.pi/contracts-review/`. Compare them with `git diff --no-index`.
3. Write the baseline only after that review: run the same command with
   `-update-contracts`. Commit the golden diff with the behavior change and a
   pinned-source citation.

The harness refuses either flag in CI, and it refuses them without a non-empty
`ASSX_CONTRACT_REVIEW`. The historical `cli_fonts` block is kept byte for byte
across a regeneration.

## SafeFix proof scope

For each fix marked `safe`, assx records every target renderer, the source edit
ranges, and the source digest. It stores one full proof for
an edit group and numbers the group; the remaining diagnostics carry that
ordinal in `fix_proof_ref`. The interpretations stay next to the edits,
including dispatch, signature citation, state, and source provenance.

`--fix` checks the source digest, then rebuilds the evidence from the current
source and the complete edit group, and compares the rebuild with the submitted
proof. It does not trust diagnostic scope labels. A subset, source, or
interpretation mismatch rejects the request. Separate proof groups also require
combined edit verification before application.

Keep a fix unavailable when a selected renderer, signature, or semantic
operation is unresolved. Keep `\\t()` unavailable because the pinned
matrix verifies only one-to-four-argument transform forms and records layout
changes for the empty form. Keep parenthesized empty `\\b` and `\\i` forms
unavailable until their signatures are verified for that exact form. Keep an
integer Style edit unavailable when the semantic Style model cannot establish
the same modeled value before and after the edit.

A custom layout cannot borrow the primary renderer's integer field proof. A proof
for VSFilterMod alone cannot authorize a default fix. `--renderer` lists
renderers in priority order: the first selects primary validity, the rest extend
comparison and proof targets. Profiles are normalized by renderer.
Compatibility validity never becomes a primary error.

The proof compares each renderer's interpretation before and after the edit. It
does not claim cross-renderer pixel equality. It does not prove behavior for
unlisted renderer versions, builds, Lua scripts, or forks.

Use the libass `rendercheck` fixture to compare exact frames after safe fixes. Use
pinned source evidence and independent renderer trace tests for xy-VSFilter. Do
not treat assx diagnostics as an independent renderer oracle. The CI rendercheck
job does not execute xy-VSFilter or VSFilterMod binaries.

## Renderer evidence

`docs/reference/ass-tags.xml` is the single owner of signatures, scenarios, and
source pins. Its header defines the status and evidence vocabulary. Do not
collapse syntactic acceptance, effective state, cross-renderer compatibility, and
safe-to-edit equivalence into one flag.

When a newly investigated source changes evidence, update that matrix and its
source-local assertions. Do not duplicate a renderer-tag table in the test
harness or in another document.

## Reproduce

From the repository root:

```sh
go test ./...
go vet ./...
go build -o dist/assx ./cmd/assx
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -count=1 -v
go test ./internal/ass/spec -run '^TestMatrix' -count=1 -v
```

For a reviewed baseline refresh only:

```sh
export ASSX_CONTRACT_REVIEW="<reviewed cause>"
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -review-contracts -count=1
git diff --no-index -- .pi/contracts-review testdata/contracts/golden
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -update-contracts -count=1
git diff -- testdata/contracts/golden
```
