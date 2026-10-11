# Code search trial

## Scope

Source commit: `a397136c6d3256b01beaa7a2d3d4f0d8ba262286`.
The working tree also contains the research guide and search configuration.
Environment: Windows amd64, Node `24.18.1`.
Tools: CodeGraph `1.6.2`, GitNexus `1.6.12`.

This trial measures returned paths and symbol coverage.
It does not measure task completion time or renderer correctness.

## CodeGraph baseline

The original configuration forced `ref/` into the main index.
The index contained 4,988 files and 133,222 nodes.
It included 4,714 C/C++ files and 118 Go files.

Run each query with `codegraph explore --max-files 5`.
The table counts source files returned under `ref/`.

| Query | Original reference files / returned files | Scoped reference files / returned files |
| --- | ---: | ---: |
| `SafeFix renderer evidence` | 3 / 5 | 0 / 5 |
| `EvaluateResolved AnalyzeNoEffectsForRenderer` | 2 / 5 | 0 / 5 |
| `CompareDialogue CompareDocument` | 1 / 5 | 0 / 5 |
| `AnalyzeDocument ApplyFixes` | 2 / 5 | 0 / 5 |
| `ConcreteDialogue Profile Resolve` | 2 / 5 | 0 / 5 |

The original index returned ten reference paths out of 25 source paths.
The scoped index returned none. It contains 123 files and 1,684 nodes.
The database size fell from 302.41 MB to 6.13 MB.
All five queries retained the named assx symbols or their owning files.
The MCP query also returned assx consumers and tests without reference paths.
This count does not assess each assx path's relevance.

## Separate renderer scope

The libass index contains 80 files and 1,801 nodes.
The following query returned `libass/ass_parse.c` first:

```sh
codegraph init ref/renderers/libass
codegraph explore --path ref/renderers/libass --max-files 3 ass_parse_tags
```

It identified `ass_parse_tags` and its caller in `libass/ass_render.c`.
The three returned source paths were all within the libass checkout.
Use a separate checkout index for renderer questions.

## GitNexus setup

GitNexus is installed globally. Repeat the installation with:

```sh
pnpm add -g gitnexus@1.6.12 --allow-build=@ladybugdb/core --allow-build=gitnexus --allow-build=tree-sitter
```

`.gitnexusignore` excludes references and tool data.
It retains Go test files. Build the index without generated instructions:

```sh
gitnexus analyze --index-only
gitnexus status
gitnexus query -r assx --limit 5 "EvaluateResolved AnalyzeNoEffectsForRenderer"
gitnexus context -r assx --file internal/semantic/effective.go EvaluateResolved
```

The trial does not use embeddings, generated skills, or editor setup.
Do not treat a graph's caller count as complete without a source check.

## GitNexus results

Indexing took 32.7 seconds. It produced 2,516 nodes, 9,208 edges,
71 clusters, and 214 flows. These counts use a different schema from CodeGraph.
Do not compare node counts as coverage scores.

| Check | Result |
| --- | --- |
| Five search queries, after FTS repair | All returned assx results. No returned path was under `ref/`. |
| `context` for `EvaluateResolved` | Found the exact symbol, callers, tests, and outgoing edges. |
| `impact`, depth 1, tests included | Found 14 direct callers. A source search also found 14 call sites. |
| `trace` from `AnalyzeNoEffectsForRenderer` to `EvaluateResolved` | Found the direct call. |

The first search run returned empty results because FTS was unavailable.
The documented repair installed FTS. `gitnexus doctor` then confirmed availability.
All five repeated queries returned results.

The combined queries did not return the `CompareDocument` or `ApplyFixes` symbols.
Exact `context` queries with their file paths found both.
CodeGraph returned their owning source files in the same probes.
GitNexus works for focused graph checks. This sample does not establish better
search coverage or shorter investigation time.

The analyzer reported omitted flows and callees due to trace limits.
Some process paths end at Go types. Treat them as structural paths.
They do not prove runtime execution or complete flow coverage.

Repeat the graph checks with:

```sh
gitnexus impact -r assx --file internal/semantic/effective.go --depth 1 --include-tests EvaluateResolved
gitnexus trace -r assx --from-file internal/lint/noeffect_renderer.go --to-file internal/semantic/effective.go AnalyzeNoEffectsForRenderer EvaluateResolved
```

For an absent FTS extension, allow its download and repair the search index.
On Windows Command Prompt:

```bat
set "GITNEXUS_LBUG_EXTENSION_INSTALL=auto"
gitnexus analyze --repair-fts --index-only
```

## Trial method

Use the same five queries and the same checkout for both tools.
CodeGraph's limit counts source files. GitNexus's limit counts processes.
Do not compare these limits as equal result sizes.

Check these outcomes:

- Returned source paths stay within the selected scope.
- Exact symbols resolve to their owning files.
- Caller views contain known consumers and tests.
- Flow views contain the relevant entry point and downstream operation.

Record misses as well as matches.
Keep scoped CodeGraph as the first tool. Use GitNexus for focused caller,
impact, or path checks. Do not run both tools for every question.
Reopen this choice if a measured task trial shows fewer misses or less work.
