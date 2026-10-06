assx checks ASS subtitle files for lint issues.

## Usage

```text
assx [--format text|json] [--fix] [--unsafe-fix] [--check-fonts] [--font-dir DIR] file.ass
```

- `file.ass`: the ASS file to check.
- `--format text|json`: choose diagnostic output format; defaults to `text`.
- `--fix`: apply safe fixes.
- `--unsafe-fix`: apply safe and unsafe fixes.
- `--check-fonts`: check whether used font families are available and contain dialogue characters. Disabled by default.
- `--font-dir DIR`: recursively scan this folder for fonts instead of searching system font folders. Used with `--check-fonts`.

Font indexing is cached under the operating system's user cache directory. Font files are still discovered on every run, so added or removed fonts take effect immediately; cached metadata is reused only while a file's size and modification time match.

For diagnostic IDs and explanations, see the [lint rules](docs/rules.md).

## Development

Requires Go 1.23 or later. From the repository root:

```sh
go test ./...
go vet ./...
go build ./cmd/assx
```

### Git hooks

Install `prek` using your preferred method (for example, `uv tool install prek`), then install the hooks configured in `.pre-commit-config.yaml`:

```sh
prek install --prepare-hooks
```
