# assx rendercheck

This helper verifies that two ASS files render to exactly the same RGBA pixels with the same libass build and font set.

It is for SafeFix regression tests. It does not use the local `ref/` directory.

CI builds libass commit `f61db567e6593df3470e91594bcd4ad2d0473aff` and uses only pinned test fonts. The renderer uses `ASS_FONTPROVIDER_NONE`.

Example:

```sh
cargo run --manifest-path tools/rendercheck/Cargo.toml -- \
  before.ass after.ass \
  --libass /path/to/libass.so \
  --fonts /path/to/fonts \
  --width 1920 --height 1080 \
  --every-frame 24000/1001
```

The tool also samples event boundaries and timing points from `\t`, `\move`, `\fad`, `\fade`, and karaoke tags.

Event timestamps follow libass's integer centisecond convention: `0:00:00.5` means 50 ms and `0:00:00.1234` means 12,340 ms. Unsupported timestamp syntax and values outside signed 32-bit components produce an error instead of being rounded or truncated. CI checks the short `.5`–`.9` interval with files that must render differently.

Exit codes:

- `0`: all sampled RGBA frames are exactly equal
- `1`: at least one sampled frame differs
- `2`: configuration, parse, load, or render error

Use `--diff-dir DIR` to write before, after, and amplified diff PNG files for mismatched frames.
