# Architecture rules

The package graph and the request paths are in [the lint pipeline](../architecture/lint-pipeline.md). Read them before you add a package or an import. The ownership and placement rules are in [compatibility](../packages/compatibility.md).

## Implementation quality

Implement the best practical design. Do not implement only the smallest change that passes the current test.

Prefer a reusable semantic model over repeated special cases.

Keep lint rules, formatter behavior, renderer semantics, and fix safety separate when they are different concepts.

Do not add a new rule when an existing rule can correctly model the same problem.

Prefer a false negative to an unsafe automatic fix.
