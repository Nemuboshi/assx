# Diagnostic reports

`internal/report` converts final lint diagnostics into presenter data. It derives fix badges only from concrete edits plus validated `FixSafety` metadata. The `plain` presenter preserves the legacy text format; `pretty` renders source frames and optional evidence. JSON still encodes `lint.Diagnostic` directly.

Pretty output uses Lip Gloss v2 **v2.0.6** for severity styling and `github.com/charmbracelet/x/ansi` **v0.11.9** for terminal cell width and grapheme-safe cropping. `github.com/rivo/uniseg` **v0.4.7** expands tabs at grapheme boundaries. Versions are pinned in `go.mod`.

Tag and dialogue-text columns are relative to the dialogue text; event and Style field positions use parser byte spans. `report.Build` normalizes these to the physical source line using `ass.Dialogue.TextStart` and `LineStart`. Tag diagnostics underline the parsed tag expression; repeated-slash findings underline the extra slashes. Missing or unmatched ranges get a point marker at the diagnostic location. The presenter never infers a source range from a proposed edit.

The pretty presenter escapes terminal control characters from file paths, source, and diagnostic text. It crops long source lines around the location and adapts the source-frame width to the detected terminal width (80 columns when output is redirected). Color is enabled only for an interactive stdout when `NO_COLOR` is unset and `TERM` is not `dumb`. Scan progress is written only to interactive stderr.

Use `--explain` to include full rule descriptions and evidence URLs. The default report shows the short diagnostic detail when available.

Build and CI use the Go version declared by `go.mod` (Go 1.27.0). CI uses `actions/setup-go` with `go-version-file: go.mod` so it does not silently select a newer toolchain.

`BenchmarkRenderReport` renders 50 findings against a 200-line source. On Windows/amd64 with Go 1.27.0, a 10-iteration sample measured about 2.72 ms/op, 238,274 B/op, and 2,649 allocs/op. Run it with `go test ./internal/report/pretty -run '^$' -bench BenchmarkRenderReport -benchmem` to compare later changes.
