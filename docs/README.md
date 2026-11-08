# assx documentation

This page is the index. Start with the document for the affected subject.

| Subject | Owner | Start here |
| --- | --- | --- |
| Command usage, flags, exit status | `cmd/assx` | [README](../README.md) |
| Package structure, lint flow, proof, and request paths | `internal/lint`, `internal/analysis` | [Lint pipeline](architecture/lint-pipeline.md) |
| Snapshot analysis and deferred proof | `internal/analysis` | [Analysis](packages/analysis.md) |
| Syntax and source spans | `internal/ass` | [Syntax](packages/syntax.md) |
| Tag signatures and dispatch | `internal/ass/spec`, `internal/ass/renderer` | [Profiles](packages/profiles.md), [tag matrix](ass-rules/ass-tags.xml) |
| Style field conversion | `internal/ass/renderer` | [Style field matrix](ass-rules/ass-style-fields.xml), [compatibility](packages/compatibility.md) |
| State, resets, and transforms | `internal/semantic` | [Semantics](packages/semantics.md) |
| Lint policy and rule IDs | `internal/lint` | [Rule catalog](ass-rules/catalog.md), [lint](packages/lint.md) |
| Renderer differences | `internal/semantic` | [Compatibility](packages/compatibility.md) |
| SafeFix proof and the regression baseline | `internal/semantic`, `internal/lint`, `internal/edit` | [Verification rules](rules/verification.md) |
| Frame comparison | `tools/rendercheck` | [Rendercheck](../tools/rendercheck/README.md) |
| Font dependencies | `internal/fontcheck`, `internal/fontreport` | [Font checking](usage/font-checking.md) |
| CLI reports | `cmd/assx`, `internal/report` | [Reports](usage/diagnostic-reports.md) |
| Editor integration | `internal/lsp`, `editors/` | [Language server](usage/lsp.md) |
| Performance | The affected package | [Performance](development/performance.md) |

The tag matrix and the Style field matrix own renderer signatures and evidence.
The verification rules own proof scope. The rule catalog owns diagnostic
policy. Tests record behavior.

## Folder layout

- `rules/` states the development rules for an agent.
- `ass-rules/` holds the ASS rule catalog and the code-generated evidence matrices.
- `usage/` states what a user sees: commands, reports, fonts, and editors.
- `packages/` states the API contract of each package.
- `architecture/` states the package graph, the lint flow, and the request paths.
- `development/` holds the benchmark harness and its baseline.

Read [AGENTS.md](../AGENTS.md) to find the development rules.