# ASS rule catalog

This page explains assx's stable diagnostic IDs. Runtime metadata—titles, descriptions, suggested fixes, severity, and source URLs—is centralized in [`internal/lint/registry.go`](../internal/lint/registry.go). All diagnostics use stable `ASSxxx` IDs whose numeric part identifies a rule without encoding severity or category. Severity is independent metadata, so it may change without changing the diagnostic ID. Keep an ID when wording, severity, fix safety, or presentation changes; introduce a new ID only when the detected condition is genuinely different.

Issue IDs are assx identifiers, not renderer error codes. Optional font checks use the same `ASSxxx` namespace as every other diagnostic.

- `error`: the CLI exits with status 1.
- `warning`: the CLI exits successfully.
- `suggestion`: the CLI exits successfully.

Invalid command-line usage and unreadable input exit with status 2. Diagnostics point to the physical ASS file line; columns are one-based within the Dialogue `Text` field.

The references below are pinned to libass [`f61db56`](https://github.com/libass/libass/tree/f61db56) and xy-VSFilter [`135a3015`](https://github.com/Masaiki/xy-VSFilter/tree/135a3015). Each section links only to the source relevant to that rule.

## ASS001 — Invalid argument count

`pos`, `move`, `org`, `fade`, `fad`, `clip`, and `iclip` accept specific argument counts. A wrong count may cause the renderer to ignore the tag. Correct the argument list. Vector clips take one or two arguments; rectangular clips take four.

References: libass [`move`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L494), [`pos`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L608), [`fade`/`fad` and `org`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L625), [vector clip](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L405), and [rectangular clip](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L730); VSFilter [clip](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2352), [fade](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2396), [move/origin](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2575), and [position](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2615).

## ASS002 — Invalid tag value

A checked value is malformed, or an integer tag is outside its accepted range. Supply a valid number or hexadecimal value. Constrained integer tags include `an`, `a`, `b`, `i`, `q`, `s`, and `u`.

References: libass [alignment](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L584), [bold/italic](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L825), and [decorations/wrap style](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L882); VSFilter [alignment](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2301), [bold](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2344), and [wrap style](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2635).

## ASS003 — Renderer behavior differs

Parenthesized `&H` color/alpha values and `blur` values above 100 can behave differently between libass and VSFilter. Choose an unambiguous form or inspect the result in both renderers.

References: libass [hex conversion](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L231), [blur clamping](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L425), and [color/alpha tags](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L749); VSFilter [color/alpha parsing](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2205) and [blur](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2315).

## ASS004 — Comma splits font name

Commas in parenthesized `fn` arguments split the arguments and may change the selected font name. Use concatenated tag syntax for names containing commas, or verify the parsed value.

References: libass [argument splitting](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L302); VSFilter [argument splitting](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2110) and [`fn` handling](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2447).

## ASS005 — VSFilterMod tag may be unsupported by libass/VSFilter

VSFilterMod adds `\1img`, `\2img`, `\3img`, `\4img`; `\1vc`–`\4vc`; `\1va`–`\4va`; `\distort`, `\frs`, `\fsvp`, `\fshp`, `\jitter`, `\lua`, `\mover`, `\moves3`, `\moves4`, `\movevc`, `\rnds`, `\rndx`, `\rndy`, `\rndz`, `\rnd`, `\xblur`, `\yblur`, `\z`, `\ortho`, and `\blend`. These tags are absent from the shared libass and original VSFilter tag set, so either renderer may ignore them or interpret their content differently. `lua` is conditional on VSFilterMod being built with Lua support. The warning has no fix, and automatic fixes are disabled for the dialogue containing the tag.

References: pinned VSFilterMod [tag handlers](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2745-L3717), original VSFilter [tag map](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1716-L1769), and libass [tag parser](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L281-L888).

## ASS006 — Override has no effect

An override is replaced or reset before it affects dialogue text, or a first-wins tag is ignored by an earlier tag. Remove it or move it to the intended text boundary. Tags inside `\t(...)` are tracked as transition values so they do not incorrectly make their starting value appear dead.

References: libass [first-wins handling](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L584) and [style reset](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L1075); VSFilter [alignment](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2301), [fade](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2396), [position/origin](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2575), and [style reset](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2643).

## ASS007 — Unknown or misplaced override tag

The override name is not recognized, or the text escape `N`, `n`, or `h` appears inside an override block. Correct the spelling, remove the unknown tag, or move the escape into dialogue text.

References: VSFilter [command map](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1716) and [tag lookup](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2156); libass [tag dispatch](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L348) and [text escape parsing](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L1119).

## ASS008 — YCbCr matrix needs review

The matrix is missing, or a 601 matrix is selected while `PlayResY` is above 576. Renderer defaults can differ and the chosen matrix affects subtitle colors. The unsafe suggestion adds `YCbCr Matrix: None` when absent, or changes TV/PC.601 to the matching 709 range for high-resolution scripts.

References: libass documents the matrix defaults and color-mangling differences in [`ass_types.h`](https://github.com/libass/libass/blob/f61db56/libass/ass_types.h#L175) and parses the header in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L351); VSFilter parses matrix values in [`STS.cpp`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1633).

## ASS009 — Layout resolution is missing

libass uses `LayoutResX` and `LayoutResY` for layout and scaling. If either is absent or non-positive, its fallback can depend on renderer settings. The unsafe suggestion fills missing or invalid values from the corresponding `PlayResX` and `PlayResY` values when both are valid.

References: libass parses the headers in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L888), chooses the effective layout resolution in [`ass_render.c`](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L1008), and uses it in [`ass_parse.c`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L939).


## ASS010 — Repeated backslash in override block

A run of backslashes before an override tag contains an extra escape character. Remove the extras while keeping the intended tag. This check is limited to override blocks; literal backslashes in dialogue text are not reported.

## ASS011 — Fractional value in integer style field

Style fields such as `Bold`, `Italic`, `Alignment`, `BorderStyle`, and `MarginL/R/V` are parsed as integers. A fractional suffix is discarded by the checked libass and VSFilter parsers. The safe fix keeps the integer prefix those parsers consume.

References: libass integer conversion in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L323) and style field dispatch in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L434); VSFilter [`GetInt`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1237) and style parsing in [`STS.cpp`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1511).

## ASS012 — Style value exceeds VSFilter float precision

There is no universal decimal-place limit for style numbers. libass parses floating fields as doubles; VSFilter reads them into 32-bit floats, which can round values beyond roughly seven significant decimal digits. This rule only reports values where the checked conversion changes the decimal value. Its unsafe suggestion rounds to VSFilter's parsed value, which may slightly change libass output.

References: libass float parsing via [`ass_atof`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L42) and style field dispatch in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L606); VSFilter [`GetFloat`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1254) and style parsing in [`STS.cpp`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1509).

## ASS013 — Font overrides match style

This rule compares `\fn`, `\fs`, `\b`, `\i`, `\fscx`, `\fscy`, and `\fsp` against the dialogue's style. It reports when those tags leave the tracked font state equal to the style for all rendered text: leading blocks may contain multiple assignments before text, and later assignments are accepted only when they leave the current value unchanged. The safe fix removes only those font tags and preserves other formatting tags. Empty-value resets to the active style are modeled; relative `\fs+` and `\fs-` forms are skipped rather than interpreted as absolute sizes. The rule also skips ambiguous styles, style resets, transforms, drawing mode, and unsupported font properties.

## ASS014 — Extra opening brace in override block

A run such as `{{\i1}` has one extra opening brace before a matched override block. libass and VSFilter parse the block through its first `}` and look for tags after the opening brace, so the extra consecutive braces do not affect the parsed tags. The safe fix removes all but the first brace. Escaped braces such as `\{` are ignored, and unmatched runs are left alone because their rendered meaning is ambiguous.

References: libass [event override parsing](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L2066-L2075) and [tag scanning](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L282-L290); VSFilter [override block parsing](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2954-L2958) and [tag scanning](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2075-L2082).

## ASS015 — Subtitle font is missing

With `--check-fonts`, this finding reports an effective font family that was not found. The checker scans system font folders by default; `--font-dir DIR` makes it scan only that folder recursively. Font checks are disabled unless `--check-fonts` is set.

## ASS016 — Font is missing subtitle characters

The selected font family exists, but does not contain glyphs for one or more characters used by dialogue text. The finding lists the missing characters. Both font findings are informational suggestions with no automatic fixes.

## Fixes

Severity and fix safety are independent metadata: a diagnostic can have a `SafeFix` or an `UnsafeFix`.

A `SafeFix` preserves the rendered output in the supported renderer scope. The checker applies these edits with `--fix`. An `UnsafeFix` may change output and requires explicit opt-in with `--unsafe-fix`; that option also applies available safe fixes. Without either option, assx only reports findings. The command ends with counts of safe, unsafe, and non-auto-fixable findings. Text output keeps individual messages short; JSON retains detailed diagnostics and edit ranges.

The two header rules are unsafe: adding or changing `YCbCr Matrix` can change colors, and setting `LayoutResX/Y` can change libass scaling and tag behavior.

Formatting is not implemented as a separate command yet. Preserve tag order by default: precedence and interactions, especially inside `\t(...)`, can make reordering change output. A future formatter should only normalize syntax when it can prove the rendered result is unchanged.

assx is not a complete ASS validator or an exact renderer emulator. Citations identify the source basis for modeled rules, not a guarantee for every malformed input.

