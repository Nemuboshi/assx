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

Exit codes:

- `0`: all sampled RGBA frames are exactly equal
- `1`: at least one sampled frame differs
- `2`: configuration, parse, load, or render error

Use `--diff-dir DIR` to write before, after, and amplified diff PNG files for mismatched frames.
