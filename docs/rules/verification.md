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

Do not run `-update-contracts` as a response to a failing test. A golden
mismatch is evidence that needs review, not a request to regenerate. Commit a
golden update with the behavior change and a pinned-source citation.

## SafeFix proof scope

For each fix marked `safe`, assx records every target renderer ID, pinned
version, build capability, source edit range, source hash, edited hash, and
interpretation trace hash. It stores one full proof for an edit group and links
each remaining diagnostic with `fix_proof_ref`. The interpretations stay next to
the edits, including dispatch, signature citation, state, and source provenance.
The proof ID hashes every proof field, including that audit metadata.

`--fix` checks the submitted ID and edited-source hash, then rebuilds the
evidence from the current source and the complete edit group. It does not trust
diagnostic scope labels. A subset, revision, capability, source, edited-hash, or
interpretation mismatch rejects the request. Separate proof groups also require
combined edit verification before application.

Keep a fix unavailable when a selected renderer, signature, build capability, or
semantic operation is unresolved. Keep `\\t()` unavailable because the pinned
matrix verifies only one-to-four-argument transform forms and records layout
changes for the empty form. Keep parenthesized empty `\\b` and `\\i` forms
unavailable until their signatures are verified for that exact form. Keep an
integer Style edit unavailable when the semantic Style model cannot establish
the same modeled value before and after the edit.

A custom layout cannot borrow the primary renderer's integer field proof. A proof
for VSFilterMod alone cannot authorize a default fix. `--renderer` selects
primary validity. `--compat` extends compatibility and proof targets. An explicit
renderer without compatibility selects only that primary. Profiles are normalized
by renderer, revision, and build, so different builds remain distinct.
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

For the deliberate baseline refresh only:

```sh
go test ./cmd/assx -run '^TestRendererBaselineContracts$' -update-contracts -count=1
git diff -- testdata/contracts/golden
```
