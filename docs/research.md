# Research

Use ASD-STE100 for repository documentation. Keep technical names exact.
Use active voice and one term for each concept.
Limit procedure sentences to 20 words. Limit descriptive sentences to 25 words.
Remove repeated text. Keep scope, units, conditions, and evidence.

## Subjects

Start with the document for the affected subject.
Some documents contain implementation records from earlier issues.
Check their stated phase before you use a rollout claim as a current contract.

| Subject | Owner | Start here | Method |
| --- | --- | --- | --- |
| Syntax and source spans | `internal/ass` | [Syntax](renderer-syntax.md) | Check raw input, node spans, and round-trip tests. |
| Tag signatures and dispatch | `internal/ass/spec`, `internal/ass/renderer` | [Profiles](renderer-profiles.md), [matrix](ass-tags.xml) | Check the exact form, source pin, and build guards. |
| State, resets, and transforms | `internal/semantic` | [Semantics](renderer-semantics.md) | Trace one renderer at a time. Check state and provenance. |
| Lint policy and rule IDs | `internal/lint` | [Rules](rules.md), [scoped lint](renderer-lint.md) | Check the existing rule and its consumers. |
| Renderer differences | `internal/semantic` | [Compatibility](renderer-compatibility.md) | Compare the same input in each selected renderer. |
| SafeFix and source edits | `internal/semantic`, `internal/lint`, `internal/edit` | [Contracts](renderer-contracts.md) | Prove each selected renderer's result before and after the edit. |
| Frame comparison | `tools/rendercheck` | [Rendercheck](../tools/rendercheck/README.md) | Compare frames in the stated renderer and build. |
| CLI reports | `cmd/assx`, `internal/report` | [Reports](diagnostic-reports.md), [usage](../README.md) | Check output, exit status, and file changes. |
| Performance | The affected package | [Performance](performance.md) | Compare the same workload, toolchain, and machine. |

The matrix owns renderer signatures and scenario evidence.
Contracts own proof scope and baseline review.
Rules own diagnostic policy. Tests record behavior.
A historical implementation record does not override these sources.
Resolve a conflict against pinned source and an independent check.
Update the affected document with the correction.

## Code search

The main CodeGraph index excludes `ref/` and tool data.
Rebuild it after a scope change:

```sh
codegraph index
codegraph status
codegraph explore --max-files 5 EvaluateResolved AnalyzeNoEffectsForRenderer
```

Use exact symbols or file names. Check the returned paths first.
If they miss the subject, narrow the query once.
Then use a targeted `rg` search or file read.
Do not repeat broad queries that return unrelated code.

For renderer research, index only the required checkout:

```sh
codegraph init ref/renderers/libass
codegraph explore --path ref/renderers/libass --max-files 3 ass_parse_tags
```

For MCP, set `projectPath` to that checkout's absolute path.
A path inside the main index does not restrict search to that directory.
Create the separate index first. Use the same method for community tools.
Keep reference checkouts and their indexes out of Git.

Use GitNexus for a focused check when CodeGraph leaves a gap.
Use `query` to find code, `context` to inspect a symbol, `impact` to find
consumers, and `trace` to find a call path. Do not run both tools by default.
Check index freshness before use. Rebuild after source changes.
Use graph results to locate source. Do not use them as behavioral proof.
See [the trial](tool-trial.md) for commands and results.

## Investigation

1. State one question. Give the input and expected behavior.
2. Select the subject, packages, renderers, and build scope.
3. Find an existing finding. Check its pins and assumptions.
4. Inspect only missing evidence and affected consumers.
5. Run the smallest independent check that can settle the question.
6. Record the conclusion. Link its evidence, test, and reopen condition.

For renderer claims, inspect pinned implementation source.
Record the exact argument form, aliases, slots, resets, transforms, and
build guards when they affect the claim. Compare each selected renderer.
Community tools can identify a case. They cannot prove renderer behavior.

For SafeFix, check the output after the complete edit group.
Keep the fix unavailable when any selected target lacks proof.
Equal modeled state does not prove equal frames.
A test of assx alone does not independently verify a renderer.

After ten minutes, record the unresolved question and the next decisive check.
Continue when correctness requires it. Stop general exploration when you have
the owner, affected consumers, sufficient evidence, and a regression check.

## Finding record

Put task-specific findings in the issue or PR.
Put reusable findings in the existing subject document or matrix.
Do not create a second renderer table or copy tool output into documentation.

```text
Question: One claim and a minimal input.
Conclusion: The result and its limits.
Status: source-verified / runtime-verified / inferred / unresolved.
Scope: assx commit, renderer commits, build flags, assumptions.
Evidence: Source permalink, symbol, and the reason it proves the claim.
Code: Owner, entry point, affected consumers.
Check: Test or fixture, command, environment, observed result.
Reopen if: Relevant source, build scope, input form, or assumption changes.
```

`source-verified` requires a direct check of pinned implementation source.
`runtime-verified` requires a reproducible run with a stated binary and build.
`inferred` records a claim without direct verification.
`unresolved` records the missing evidence. State which check can resolve it.
These labels describe the finding. They do not replace matrix classifications.
Record source and runtime evidence separately when both exist.

Reuse a finding only within its stated scope.
Check cited source when the change depends on it. Do not repeat unrelated
renderer research when the pins and assumptions still apply.
Update evidence and tests with the behavior change.

## Document review

Keep current behavior in the subject document.
Keep issue history in a section marked `Implementation record`.
Replace obsolete rollout claims when the next phase is complete.
Link to the owning document instead of copying its contract.

Check technical terms, source links, paths, commands, and status labels.
Review ASD-STE100 rules and vocabulary. Short text alone does not establish
ASD-STE100 compliance.
