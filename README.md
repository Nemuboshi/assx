assx checks ASS subtitle files for lint issues.

## Usage

```text
assx [lint] [--renderer=libass|xy-vsfilter|vsfiltermod] [--compat=RENDERERS] [--mod-build=unknown|enabled|disabled] [--format pretty|plain|json] [--explain] [--fix] [--unsafe-fix] [--force-ass] file.ass
```

- `file.ass`: the ASS file to check.
- `--renderer`: select the primary pinned renderer for validity checks.
- `--compat`: add comma-separated profiles for compatibility and SafeFix checks. Use an empty value to select only the primary.
- Without either flag, assx selects libass as primary and xy-VSFilter for compatibility.
- An explicit `--renderer` without `--compat` selects only that primary. Compatibility warnings never become primary validity errors.
- `--mod-build`: declare the VSFilterMod `_VSMOD` capability. It defaults to `unknown`; assx never infers it. The flag requires VSFilterMod as primary or in `--compat`. A known Mod capability is required for Mod SafeFix proofs.
- assx does not model the VSFilterMod `_LUA` gate. The pinned source compiles Lua code but loads no script. See [build capabilities](docs/architecture/profiles.md#vsfiltermod-build-capabilities).
- Both `--flag` and `-flag` forms work, with separate values or `=value`.
- Exit status: 0 for no primary errors, 1 for primary errors, 2 for usage, input/output, or fix-application failures.
- `--format pretty|plain|json`: choose output format; defaults to `pretty`. Pretty output shows a compact diagnostic report with source frames; color is used only on an interactive terminal. A scan spinner appears only when stderr is interactive. `plain` preserves the text-oriented output; `json` emits machine-readable diagnostics.
- `--explain`: include expanded rule descriptions and evidence URLs in pretty output.
- `--fix`: apply safe fixes. If the input is a symbolic link, update its target and preserve the link.
- `--unsafe-fix`: apply safe and unsafe fixes.
- `--force-ass`: bypass content recognition for intentionally partial input. This can permit writes with `--fix` or `--unsafe-fix`. It does not bypass encoding or SafeFix checks.

assx checks content before ASS analysis. A `.ass` or `.ssa` suffix does not prove the file type. A file with another suffix can still contain ASS/SSA. Credible ASS/SSA records in their sections permit normal analysis and fixes. Sparse structure, such as an empty `[Events]` section, permits analysis but not writes. The CLI prints a notice on stderr. Foreign or unrecognized input causes an input error (exit 2), with no ASS lint report or fixes. `--force-ass` overrides only this recognition decision. Lint errors in recognized input return exit 1. See [input recognition](docs/architecture/syntax.md#input-recognition).


Font checks are separate from lint:

```sh
assx fonts check [--force-ass] file.ass
assx fonts check --font-dir ./fonts --format json file.ass
```

Font checks return advisory `FONT001` and `FONT002` findings. They do not change lint counts, fixes, or exit status.
The font command returns 0 when checks complete, or 2 for usage and operational failures.
See [font checking](docs/usage/font-checking.md) for catalog scope, JSON, cache behavior, and limits.

`assx lsp` starts an LSP 3.17 server on stdio. It provides diagnostics, hover, signature help, Style navigation, symbols, and completion for in-memory ASS text. SafeFix code actions require versioned edit support. The [VS Code adapter](editors/vscode) launches the binary; [Neovim](editors/neovim/init.lua) and [Helix](editors/helix/languages.toml) examples configure other clients. See [LSP usage and limits](docs/usage/lsp.md).

All analysis uses the lossless concrete syntax tree and pinned profile engine.
SafeFix application recomputes authorized edits under the caller's primary and compatibility policy.
See [the evidence contract](docs/architecture/contracts.md) for evidence scope and limits.

For diagnostic IDs and explanations, see the [rule catalog](docs/ass-rules/catalog.md). For pretty report behavior, see [diagnostic reports](docs/usage/diagnostic-reports.md).
For reproducible performance benchmarks, profiling, and the pinned baseline, see [performance benchmarks](docs/development/performance.md).

## Development

Use the [documentation index](docs/README.md) to find subject documents.
Use ASD-STE100 for repository documentation. Keep technical names and evidence exact.

Requires Go 1.27.0 or later. From the repository root:

```sh
go test ./...
go vet ./...
go build -o dist/assx ./cmd/assx
```

### Static analysis

Use `golangci-lint` v2.14.0 with [.golangci.yml](.golangci.yml). It scans the
entire Go module, including tests, with `errcheck`, `govet`, `ineffassign`,
`staticcheck`, and `unused`. Local hooks and CI share this configuration and
report findings without applying automatic fixes.

With the pinned binary installed:

```sh
golangci-lint config verify
golangci-lint run
```

Keep the version pins in `.pre-commit-config.yaml` and
`.github/workflows/ci.yml` in sync when upgrading.

### Git hooks

Install `prek` using your preferred method (for example, `uv tool install prek`), then install the hooks configured in `.pre-commit-config.yaml`:

```sh
prek install --prepare-hooks
```

`prek` installs the pinned linter in its managed Go environment. A global
`golangci-lint` installation is unnecessary for hook users. Run checks manually:

```sh
prek run golangci-lint-full --all-files
prek run golangci-lint-config-verify --all-files
```

The full-module lint hook also runs for configuration-only commits.
