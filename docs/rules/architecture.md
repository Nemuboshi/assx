# Architecture rules

The package boundaries and dependency rules are in [the package architecture](../architecture/overview.md). Read them before you change packages or APIs.

## Implementation quality

Implement the best practical design. Do not implement only the smallest change that passes the current test.

Prefer a reusable semantic model over repeated special cases.

Keep lint rules, formatter behavior, renderer semantics, and fix safety separate when they are different concepts.

Do not add a new rule when an existing rule can correctly model the same problem.

Prefer a false negative to an unsafe automatic fix.
