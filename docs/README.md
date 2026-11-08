# assx documentation

This page is the index. Start with the document for the affected subject.

| Subject | Owner | Start here |
| --- | --- | --- |
| Command usage, flags, exit status | `cmd/assx` | [README](../README.md) |
| Snapshot analysis and deferred proof | `internal/analysis` | [Analysis](architecture/analysis.md) |
| Syntax and source spans | `internal/ass` | [Syntax](architecture/syntax.md) |
| Tag signatures and dispatch | `internal/ass/spec`, `internal/ass/renderer` | [Profiles](architecture/profiles.md), [tag matrix](rules/ass-tags.xml) |
| Style field conversion | `internal/ass/renderer` | [Style field matrix](rules/ass-style-fields.xml), [compatibility](architecture/compatibility.md) |
| State, resets, and transforms | `internal/semantic` | [Semantics](architecture/semantics.md) |
| Lint policy and rule IDs | `internal/lint` | [Rule catalog](rules/catalog.md), [lint](architecture/lint.md) |
| Renderer differences | `internal/semantic` | [Compatibility](architecture/compatibility.md) |
| SafeFix and source edits | `internal/semantic`, `internal/lint`, `internal/edit` | [Evidence contract](architecture/contracts.md) |
| Frame comparison | `tools/rendercheck` | [Rendercheck](../tools/rendercheck/README.md) |
| Font dependencies | `internal/fontcheck`, `internal/fontreport` | [Font checking](usage/font-checking.md) |
| CLI reports | `cmd/assx`, `internal/report` | [Reports](usage/diagnostic-reports.md) |
| Editor integration | `internal/lsp`, `editors/` | [Language server](usage/lsp.md) |
| Performance | The affected package | [Performance](development/performance.md) |
| Evidence method | All packages | [Research](development/research.md) |

The tag matrix and the Style field matrix own renderer signatures and evidence.
The evidence contract owns proof scope. The rule catalog owns diagnostic
policy. Tests record behavior.

## Folder layout

- `usage/` states what a user sees: commands, reports, fonts, and editors.
- `architecture/` states package boundaries and invariants.
- `rules/` holds the rule catalog and the code-generated evidence matrices.
- `development/` holds the research method and the benchmark harness.

## Document rules

Use ASD-STE100. Keep technical names exact.
Use one term for each concept. Use short sentences and active voice.

State current behavior only. State decisions, not the stages that produced them.
Put stage history, issue links, and past measurements in the issue or the pull
request. Link to the owning document instead of copying its contract.

Resolve a conflict against pinned source and an independent check.
Update the affected document with the correction.

Read [Research](development/research.md) before code analysis.