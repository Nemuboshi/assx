# Workflow rules

## General

- Use ASD-STE100 for English text.
- Use short sentences and approved words.
- Keep the code simple.
- Make only the changes that the task requires.
- Do not change code that is not part of the task.
- Do not add code or dependencies without a reason.
- Do not change existing behavior without a requirement.
- Do not make assumptions. Get the necessary data.

## Verification

- Do the applicable tests and static checks.
- Do a race test after changes to concurrent code.
- Use benchmarks to measure performance changes.
- Check that the intended result is the same before and after an edit.
- Add a regression test for every fix defect.
- Examine the final diff.
- Do not ignore errors.
- Do not change tests to conceal defects.

## Report

- Give a short description of the changes.
- Give the test results.
- Identify errors and tests that you did not do.
- Identify risks that remain.
- Use short, factual sentences.
