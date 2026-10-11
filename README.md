assx checks ASS subtitle files for lint issues.

## Usage

```text
assx [--format pretty|plain|json] [--explain] [--fix] [--unsafe-fix] [--check-fonts] [--font-dir DIR] file.ass
```

- `file.ass`: the ASS file to check.
- `--format pretty|plain|json`: choose output format; defaults to `pretty`. Pretty output shows a compact diagnostic report with source frames; color is used only on an interactive terminal. A scan spinner appears only when stderr is interactive. `plain` preserves the text-oriented output; `json` emits machine-readable diagnostics.
- `--explain`: include expanded rule descriptions and evidence URLs in pretty output.
- `--fix`: apply safe fixes. If the input is a symbolic link, update its target and preserve the link.
- `--unsafe-fix`: apply safe and unsafe fixes.
- `--check-fonts`: check whether used font families are available and contain dialogue characters. Disabled by default.
- `--font-dir DIR`: recursively scan this folder for fonts instead of searching system font folders. Used with `--check-fonts`.


Font indexing is cached under the operating system's user cache directory. Font files are still discovered on every run, so added or removed fonts take effect immediately; cached metadata is reused only while a file's size and modification time match.

For diagnostic IDs and explanations, see the [lint rules](docs/rules.md). For pretty report behavior and dependency versions, see [diagnostic reports](docs/diagnostic-reports.md).
For reproducible performance benchmarks, profiling, and the pinned baseline, see [performance benchmarks](docs/performance.md).

## Development

Use [Research](docs/research.md) to find subject documents, select tools, and record evidence.
Use ASD-STE100 for repository documentation. Keep technical names and evidence exact.

Requires Go 1.27.0 or later. From the repository root:

```sh
go test ./...
go vet ./...
go build ./cmd/assx
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
