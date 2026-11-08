# Research rules

## Research

State one question. Give the input and expected behavior. Then:

1. Select the subject, packages, renderers, and build scope.
2. Find an existing finding. Check its pins and assumptions.
3. Inspect only missing evidence and affected consumers.
4. Run the smallest independent check that can settle the question.
5. Record the conclusion. Link its evidence, test, and reopen condition.

For renderer claims, inspect pinned implementation source. Record the exact argument form, aliases, slots, resets, transforms, and build guards when they affect the claim. Compare each selected renderer. Community tools can identify a case. They cannot prove renderer behavior.

For SafeFix, check the output after the complete edit group. Keep the fix unavailable when any selected target lacks proof. Equal modeled state does not prove equal frames. A test of assx alone does not independently verify a renderer.

After ten minutes, record the unresolved question and the next decisive check. Continue when correctness requires it. Stop general exploration when you have the owner, affected consumers, sufficient evidence, and a regression check.

Put task-specific findings in the issue or PR. Put reusable findings in the existing subject document or matrix. Do not create a second renderer table or copy tool output into documentation. Use this record:

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

`source-verified` requires a direct check of pinned implementation source. `runtime-verified` requires a reproducible run with a stated binary and build. `inferred` records a claim without direct verification. `unresolved` records the missing evidence. State which check can resolve it. These labels describe the finding. They do not replace matrix classifications. Record source and runtime evidence separately when both exist.

Update evidence and tests with the behavior change.

## Renderer behavior

When a rule depends on rendering behavior, inspect the local source code of:

- `ref/renderers/libass`
- `ref/renderers/xy-VSFilter`
- `ref/renderers/VSFilterMod`

Do not rely only on documentation or community scripts.

Compare renderer behavior. Use the comparison to find missing assx checks and unsafe assumptions.

Prefer pinned source references when you document renderer behavior.
