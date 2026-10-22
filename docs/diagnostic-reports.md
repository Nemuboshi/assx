# Diagnostic reports

`internal/lintreport` converts final lint diagnostics into the `internal/report` presentation model. It derives fix badges only from concrete edits plus validated `FixSafety` metadata. The `plain` and `pretty` presenters display pinned renderer scope and build capabilities.
`pretty` renders source frames and optional evidence. JSON encodes `lint.Diagnostic` directly, including its `renderer` scope.

Pretty output uses Lip Gloss v2 **v2.0.6** for severity styling and `github.com/charmbracelet/x/ansi` **v0.11.9** for terminal cell width and grapheme-safe cropping. `github.com/rivo/uniseg` **v0.4.7** expands tabs at grapheme boundaries. Versions are pinned in `go.mod`.

Tag and dialogue-text columns are relative to the dialogue text; event and Style field positions use parser byte spans. `lintreport.Build` normalizes these to the physical source line using `ass.Dialogue.TextStart` and `LineStart`. Tag diagnostics, including repeated-slash findings, underline the concrete expression.
Compatibility diagnostics use physical-line columns and are not offset again by the Dialogue Text start.
Their `field` identifies the comparison dimension. `source_span` supplies an absolute decoded-byte range.
Standalone compatibility maps this range through the Dialogue Text offset.
Scope text does not affect source spans.
Fix badges and counts validate proof identity and resolve grouped proof references, including after JSON round trips. Missing or unmatched ranges get a point marker at the diagnostic location. The presenter never infers a source range from a proposed edit.

Lint diagnostics carry decoded-byte `SourceSpan` ranges at their creation sites. New ranges do not change legacy JSON fields. Compatibility `source_span` remains in JSON and survives a JSON round trip. The presenter uses a span only when it lies on the reported physical line. Otherwise, it keeps the previous CLI caret location.

The pretty presenter escapes terminal control characters from file paths, source, and diagnostic text. It crops long source lines around the location and adapts the source-frame width to the detected terminal width (80 columns when output is redirected). Color is enabled only for an interactive stdout when `NO_COLOR` is unset and `TERM` is not `dumb`. Scan progress is written only to interactive stderr.

Use `--explain` to include full rule descriptions and evidence URLs. The default report shows the short diagnostic detail when available.

Build and CI use the Go version declared by `go.mod` (Go 1.27.0). CI uses `actions/setup-go` with `go-version-file: go.mod` so it does not silently select a newer toolchain.

`BenchmarkRenderReport` renders 50 findings against a 200-line source. On Windows/amd64 with Go 1.27.0, a 10-iteration sample measured about 2.72 ms/op, 238,274 B/op, and 2,649 allocs/op. Run it with `go test ./internal/report/pretty -run '^$' -bench BenchmarkRenderReport -benchmem` to compare later changes.

## Font reports

`internal/report` and its presenters do not import lint or font analysis.
`internal/fontreport` maps font findings to shared source-frame views and a separate JSON model.
Font reports contain no edit metadata or fix counts. See [font checking](font-checking.md) for the schema and migration.
