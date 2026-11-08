# Code analysis rules

## Code analysis

- Use the CodeGraph MCP tool `codegraph_explore`, or the `codegraph` CLI, for symbols, callers, and call paths. The main index excludes `ref/`. Use a separate index for a reference project.
- Use the gopls MCP tools for Go symbols, types, references, and diagnostics. Start with `go_workspace`.
- Use `ast-grep` for exact syntax node shape and count. Use `rg` for plain text, comments, and XML. `rg` skips `ref/` from the repository root because `ref/` is gitignored. Name `ref/` as a path or add `--no-ignore` to include the reference checkouts.
- Read only the necessary files and code sections. Start with at most five files.
- A graph result locates source. It does not prove renderer behavior.
- Do not do the same work again without a reason.

## Code inspection

Read [the repository README](../../README.md) before code analysis. It names the folder that owns each subject.

Reuse a finding only when its source revision, build scope, and assumptions still apply. Read only the cited source and affected callers needed for the change. Record new evidence and the condition that requires another check.

## Tools

- Use the applicable MCP tools and Skills.
- Use RTK to decrease command output. The binary is `~/.local/bin/rtk`. Add that directory to `PATH` when the shell cannot find it.
- Do not use two tools for the same task without a reason.
- Do not use subagents for simple tasks.

## AST search

`ast-grep` matches syntax nodes, not text. It separates code from comments and strings. It supports Go, C, C++, Rust, and JSON. Use `ast-grep`. The `sg` name is deprecated.

- Search one pattern in a package: `ast-grep run -p '<pattern>' -l go internal`.
- Search a renderer checkout: `ast-grep run -p '<pattern>' -l cpp ref/renderers/xy-VSFilter/src`.
- List one file's symbols: `ast-grep outline <file>`.
- Emit machine-readable matches: add `--json`.
- Rewrite matches: add `-r '<rewrite>' -U`. Review the diff before you commit.
- Find many shapes: write a rule and run `ast-grep scan -r <rule.yml> <paths>`.

A pattern cannot cross files or prove a caller.

Go caveat: `$S.Method($$$A)` can parse as a type conversion and match nothing. Add a second argument or use a rule with `kind: call_expression` and `has`.

## JSON output

Use `jq` to read JSON output. Do not write a script for one field. Inputs are `assx --format json`, `testdata/contracts/golden/*.json`, `ast-grep --json`, and CodeGraph output.

- List primary errors: `assx --format json f.ass | jq -r '.[] | select(.severity=="error") | "\(.line) \(.id)"'`.
- Count by severity: `assx --format json f.ass | jq -r 'group_by(.severity)[] | "\(.[0].severity) \(length)"'`.
- Count safe fixes: `... | jq '[.[] | select(.fix_safety=="safe")] | length'`.
- Map ast-grep matches to lines: `ast-grep run ... --json | jq -r '.[] | "\(.file):\(.range.start.line+1)"'`.
- Assert one golden field: `jq -e '.lossless_round_trip' testdata/contracts/golden/x.ass.json`.

`jq` filters output only. It does not edit files or prove renderer behavior.
