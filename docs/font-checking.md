# Font checking

Use `assx fonts check file.ass` to check font dependencies.
Use `--font-dir ./fonts` to scan one directory and its child directories.
An explicit directory never falls back to system fonts.

```sh
assx fonts check --font-dir ./fonts --format json file.ass
assx fonts check --renderer xy-vsfilter file.ass
assx fonts check --renderer vsfiltermod --mod-build enabled --lua-build disabled file.ass
```

The default primary renderer is libass. The report includes the pinned revision and build capabilities.
Font checks use one primary renderer. They do not accept `--compat`, `--fix`, or `--unsafe-fix`.
The command does not change the ASS file or font files. Catalog scans can update the metadata cache.

## Results

| Code | Meaning |
| --- | --- |
| `FONT001` | The requested family is absent from the scanned font set. |
| `FONT002` | The selected font faces lack one or more requested characters. |

Results are advisory. Completed checks return exit 0, including checks with findings or unknown semantic state.
Usage errors, source read errors, unavailable directories, and unreadable font entries return exit 2.
An incomplete catalog produces no findings. The error identifies the first unreadable entry and the total count.
Unknown font state does not produce a finding. Reports list the number of unchecked text runs.
JSON also includes those runs and their property certainty.

## Report contract

All formats identify the font catalog and primary renderer. Use `--format pretty`, `plain`, or `json`.
Pretty output shares source frames and terminal width handling with lint. Font output has no fix summary.

JSON version 1 is an object with these fields:

- `domain`: `font`.
- `version`: `1`.
- `file`: input path.
- `renderer`: name, pinned revision, and Mod/Lua build capabilities.
- `catalog`: `system` or `directory` mode, plus the exclusive directory when specified.
- `findings`: font IDs, informational severity, details, usages, missing characters, and selected faces.
- `unchecked`: text runs with unknown effective state.
- `limit`: the coverage limitation.

Each usage records the requested family, bold/italic flags, certainty, text, Style, and proven tag owners.
`column` is a one-based byte column within Dialogue Text. `source_column` is relative to the physical source line.
`source_span` is a half-open byte range in decoded UTF-8 source, not the original UTF-16 file.
The span includes ASS text escapes. `text` contains the characters submitted to glyph lookup.
Equal requests on one dialogue line share a finding. All affected text runs retain their own spans.
Matched faces record a path, collection index, and detected bold/italic flags.

## Limits

Family matching ignores case and outer spaces. It preserves punctuation and regional suffixes.
The catalog uses typographic family, family, and full names. TTC files also use their filename stem.
Style selection minimizes bold/italic differences. Equal scores use path and face order.
Equal-score faces in the selected collection contribute coverage together. Other files do not provide fallback.
This is a selection heuristic. It does not reproduce synthetic styles or renderer-specific font selection.

Glyph lookup does not test shaping, fallback, layout, or rendered pixels.
A missing glyph is not proof of a visible replacement box.
Drawing text is excluded. Unknown renderer state remains unchecked, including unresolved resets and time-dependent font state.
Font checks retain the delivered semantic and text-escape behavior. They add no new renderer model.

System directories differ on Windows, macOS, and Linux. Results describe that scanned set, not every installed font provider.
The cache uses file size and modification time. Changes that preserve both can leave stale metadata.
The cache contains metadata, not glyph maps. Selected files are read for coverage in each process.
Discarding the cache is safe. Cache write failure does not change scan results.

## Migration phase

F02 retains `--check-fonts` and lint-scoped `--font-dir` for output comparison only.
The temporary path still emits `ASS015` and `ASS016`. The independent command emits only `FONT001` and `FONT002`.
F03 removes the temporary path. Do not use the lint JSON array as the font JSON schema.
