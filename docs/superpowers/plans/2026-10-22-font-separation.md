# Font Separation Implementation Plan

> **For agentic workers:** Use executing-plans. Track each phase and its checks here.

**Goal:** Complete issues #33, #34, and #35 in order for epic #32.

**Architecture:** Keep the parser and renderer state engine unchanged. Put catalog I/O, font requests, and coverage in `internal/fontcheck`. Adapt results at the CLI boundary. Keep font reports separate from lint reports.

**Tech Stack:** Go 1.27, the existing flag parser, sfnt, and report presenters. No new dependencies.

**Spec:** GitHub issues Nemuboshi/assx#32, #33, #34, and #35.

## Global Constraints

- Preserve #16 baseline files and renderer semantics.
- Preserve legacy optional output through F02.
- Keep explicit font directories isolated from system fonts.
- Do not claim fallback, shaping, or pixel equivalence from glyph lookup.
- Keep font results advisory. Operational errors return exit 2.
- Do not renumber other lint rules.

## Review Focus

- Unknown font state must not produce a certain finding.
- Source ranges use decoded byte offsets, including non-ASCII text.
- A corrupt or unreadable catalog must not prove family absence.
- Cache reuse must not hide changed files or cross catalog boundaries.
- Removed flags must fail with migration guidance, without edits.

## F01: Domain Extraction

Files: `internal/fontcheck/{catalog,cache,coverage,usage,analyze}.go`, domain tests, and `cmd/assx/legacy_fonts.go`.

Interfaces: `Source{Directory string}`, `NewCatalog(Source) (*Catalog, error)`, `ExtractUsages(ass.Document, renderer.Profile) []Usage`, and `Analyze(ass.Document, *Catalog, renderer.Profile) ([]Finding, error)`.

- [ ] Add tests for typed requests, spans, unknown state, catalog isolation, and cache invalidation. Check failure first.
- [ ] Move the existing engine. Keep request extraction separate from I/O and coverage.
- [ ] Add a temporary CLI adapter for legacy IDs, details, scope, and order.
- [ ] Run unchanged CLI baseline tests and domain tests. Run tests, vet, build, and benchmarks.
- [ ] Commit F01 separately.

## F02: Independent Command and Reports

Files: `cmd/assx/fonts.go`, `internal/fontreport`, report view/adapter files, CLI tests, and `docs/font-checking.md`.

Interfaces: `runFonts(args []string, stdout, stderr io.Writer) int`. Font JSON carries domain, catalog, renderer, usage, matched faces, findings, and unknown coverage.

- [ ] Add failing CLI tests for commands, formats, exits, and invalid flag scopes.
- [ ] Add `fonts check` and `lint` dispatch. Reuse renderer flag validation.
- [ ] Separate the lint report adapter from presentation primitives. Add domain-specific font JSON and source frames.
- [ ] Test empty/corrupt catalogs, output errors, Unicode, deterministic order, and read-only behavior.
- [ ] Document the command and matching limits. Run all checks and commit F02 separately.

## F03: Cutover

Files: CLI flags, lint registry, baseline test harness, new cutover tests, README, rules, and report documentation.

- [ ] Add failing tests for removed flags and font-free lint output in all formats.
- [ ] Remove the temporary adapter and active ASS015/ASS016 rules.
- [ ] Preserve historical JSON snapshots. Compare unchanged baseline fields; test new font behavior separately.
- [ ] Publish command, ID, JSON, and failure-policy migration tables.
- [ ] Run tests, vet, build, race checks, rendercheck, and benchmark comparisons. Inspect the diff and obtain a fresh review.
- [ ] Submit three reviewable changes. Do not close incomplete issues.

## Execution Record

- Baseline: `ae5e147`; clean worktree. `go test ./...`: 1156 tests passed in 14 packages.
- #16 and #24 are closed. P10 merged as PR #54, commit `564a898`.
- CodeGraph CLI index is current. CodeGraph and gopls MCP tools are not available in this session.
- Use a feature branch in the current checkout. No worktree or new CLI dependency is required for this change.

- F01: domain tests and unchanged CLI baseline pass. Full suite: 1164 tests in 15 packages. Vet, build, and font race tests pass. Catalog scan: 0.89-1.15 ms/op, 113 allocs/op (warm metadata, one Go Regular file).
