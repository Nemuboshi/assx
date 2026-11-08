# Package architecture

This page states the package boundaries and the dependency rules.
Each package details its API contract in its own page under `packages/`.

## Ownership

Keep syntax, semantics, lint policy, and rendering verification separate.

- `internal/ass` owns ASS file parsing and the lossless dialogue syntax tree.
- `internal/ass/spec` owns shared override tag definitions and renderer-facing tag metadata.
- `internal/semantic` owns effective state, Style resolution, and reusable ASS semantics.
- `internal/edit` owns generic source edits and overlap-safe edit application.
- `internal/lint` owns rule IDs, diagnostics, lint policy, and SafeFix/UnsafeFix decisions.
- `tools/rendercheck` verifies rendering equivalence. It does not define ASS semantics.

## Dependency rules

Do not put reusable parser or state logic in `internal/lint`.

The formatter must consume `internal/ass` and `internal/semantic`. It must not depend on `internal/lint`.

Preserve unknown syntax and raw source spans in the AST. Do not discard data only because assx does not understand it yet.
