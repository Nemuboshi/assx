# Architecture rules

## Architecture

Keep syntax, semantics, lint policy, and rendering verification separate.

- `internal/ass` owns ASS file parsing and the lossless dialogue syntax tree.
- `internal/ass/spec` owns shared override tag definitions and renderer-facing tag metadata.
- `internal/semantic` owns effective state, Style resolution, and reusable ASS semantics.
- `internal/edit` owns generic source edits and overlap-safe edit application.
- `internal/lint` owns rule IDs, diagnostics, lint policy, and SafeFix/UnsafeFix decisions.
- `tools/rendercheck` verifies rendering equivalence. It does not define ASS semantics.

Do not put reusable parser or state logic in `internal/lint`.

The formatter must consume `internal/ass` and `internal/semantic`. It must not depend on `internal/lint`.

Preserve unknown syntax and raw source spans in the AST. Do not discard data only because assx does not understand it yet.

## Implementation quality

Implement the best practical design. Do not implement only the smallest change that passes the current test.

Prefer a reusable semantic model over repeated special cases.

Keep lint rules, formatter behavior, renderer semantics, and fix safety separate when they are different concepts.

Do not add a new rule when an existing rule can correctly model the same problem.

Prefer a false negative to an unsafe automatic fix.
