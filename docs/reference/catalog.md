# ASS condition catalog

This page explains assx's diagnostic conditions. Runtime metadata — titles,
descriptions, suggested fixes, base severity, and source URLs — is centralized
in [`internal/lint/registry.go`](../../internal/lint/registry.go).

A condition names the fact that was detected, independently of which renderer
noticed it. One finding carries a per-renderer observation list and a derived
severity:

- `condition` is the condition name below. It is also the configuration key.
- `scenario` refines the condition when the same fact has distinct causes.
- `severity` is derived: an error-class condition is an `error` only when the
  primary renderer rejects it; otherwise it is a `warning`. Decorative
  conditions stay `suggestion`.
- `renderers` records a status for each selected renderer that decided the
  condition (`accepted`, `rejected`,
  `ignored`, `unknown`, `unmodeled`, `inferred`).

`error` exits with status 1. `warning` and `suggestion` exit successfully.
Invalid command-line usage and unreadable input exit with status 2. Diagnostics
point to the physical ASS file line; columns are one-based and point into
Dialogue `Text` for override tags or the referenced field for header, style, and
Dialogue style-reference findings.

The references below are pinned to libass
[`f61db56`](https://github.com/libass/libass/tree/f61db56) and xy-VSFilter
[`135a3015`](https://github.com/Masaiki/xy-VSFilter/tree/135a3015). Each section
links only to the source relevant to that condition.

## arity — Invalid argument count

`pos`, `move`, `org`, `fade`, `fad`, `clip`, and `iclip` accept specific argument counts. A wrong count may cause the renderer to ignore the tag. Correct the argument list. Vector clips take one or two arguments; rectangular clips take four.

References: libass [`move`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L494), [`pos`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L608), [`fade`/`fad` and `org`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L625), [vector clip](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L405), and [rectangular clip](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L730); VSFilter [clip](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2352), [fade](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2396), [move/origin](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2575), and [position](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2615).

## value — Invalid tag value

A checked value is malformed, or an integer tag is outside its accepted range. Supply a valid number or hexadecimal value. Constrained integer tags include `an`, `a`, `b`, `i`, `q`, `s`, and `u`.

References: libass [alignment](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L584), [bold/italic](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L825), and [decorations/wrap style](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L882); VSFilter [alignment](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2301), [bold](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2344), and [wrap style](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2635).

## dispatch — Renderer dispatch differs

Parenthesized `&H` color/alpha values, `blur` values above 100, some fractional rectangular `\clip`/`\iclip` coordinates, and legacy `\a4`/`\a8` alignment values can behave differently between renderers. For rectangular clips, libass and VSFilterMod consume integer coordinates while xy-VSFilter adds `0.5` before integer conversion. For `\a`, libass maps any value with `val & 3 == 0` (including 4 and 8) to middle-center like `\a5` (`ass_parse.c:595-604`), while both VSFilter forks compute `((n-1)&3)+1 + (n&4?6:0) + (n&8?3:0)` (`RTS.cpp:2308`, `VSFilterMod RTS.cpp:2909`), producing different alignments. Choose an unambiguous form or inspect the result in the target renderers.

References: libass [hex conversion](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L231), [rectangular `iclip`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L404-L423), [blur clamping](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L425), [rectangular `clip`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L729-L744), [`\a` handling](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L595-L604), [`\t` handling](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L690-L717), and [color/alpha tags](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L749); xy-VSFilter [color/alpha parsing](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2205), [blur](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2315), [`\a` handling](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2308-L2313), [`\t` handling](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2670-L2698), [render-time `\move` window](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L3255-L3256), and [rectangular clips](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2352-L2382); VSFilterMod [rectangular clips](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2953-L2998) and [`\a` handling](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2909-L2914).

## arguments — Renderer argument split differs

The selected renderers split or consume one construct's arguments differently.
The finding records each renderer's observation; the scenario names the
dimension. No automatic fix is available.

## signature — Renderer signature acceptance differs

One selected renderer accepts an argument form that another rejects or leaves
unresolved. The finding records each renderer's status.

## state — Renderer state differs

The selected renderers derive different effective state from one construct. The
scenario names the comparison dimension.

## ownership — Renderer ownership differs

The selected renderers assign one construct to different owning operations.

## application — Renderer application differs

The selected renderers apply one construct at different times or in different
scopes.

## layout — Renderer layout differs

The selected renderers read or lay out one record differently, for example a
Style row with a short Format line.

## font-comma — Comma splits font name

Commas in parenthesized `fn` arguments split the arguments and may change the selected font name. Use concatenated tag syntax for names containing commas, or verify the parsed value.

References: libass [argument splitting](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L302); VSFilter [argument splitting](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2110) and [`fn` handling](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2447).

## portability — VSFilterMod tag may be unsupported by libass/VSFilter

VSFilterMod adds `\1img`, `\2img`, `\3img`, `\4img`; `\1vc`–`\4vc`; `\1va`–`\4va`; `\distort`, `\frs`, `\fsvp`, `\fshp`, `\jitter`, `\lua`, `\mover`, `\moves3`, `\moves4`, `\movevc`, `\rnds`, `\rndx`, `\rndy`, `\rndz`, `\rnd`, `\xblur`, `\yblur`, `\z`, `\ortho`, and `\blend`. These names are not portable to libass and xy-VSFilter.
portability remains active when the requested target set excludes Mod.
It checks verified Mod name dispatch, not invocation validity or signature acceptance.
Primary profiles still report unknown names, invalid shorter-prefix values, or unresolved forms separately.
`\lua` resolves only in an enabled Mod build. assx does not model its effect. The warning has no edit.
Unknown operations and prefix collisions remain SafeFix barriers.

References: pinned VSFilterMod [tag handlers](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2745-L3717), original VSFilter [tag map](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1716-L1769), and libass [tag parser](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L281-L888).

## no-effect — Override has no effect

An override is replaced or reset before it affects dialogue text, repeats the known value already active in every affected state slot, or loses a first-wins slot to an earlier tag. Numeric forms are compared by renderer-consumed value, and aliases share their semantic slots. Relative font-size operations are not treated as absolute assignments. A `\t(...)` transform is only reported when its modeled targets cannot change the known render state and collision participation is already disabled by an earlier positioning, origin, or transform operation. Empty or same-value transforms on ordinary unpositioned lines are preserved because the transform itself disables collision layout. A transform that can change state invalidates later same-value proofs until the state is known again. Unknown tags stop later no-effect proofs because renderers may recognize a prefix that assx does not model. Parenthesized `&H` color/alpha values are excluded from shared-value proofs; combined `\shad` values are compared after their zero clamp, while axis-specific shadow values retain their sign.

References: libass [first-wins handling](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L584), [`pos` collision handling](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L608-L618), [`org` and `t` collision handling](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L655-L704), and [style reset](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L1075); xy-VSFilter [`t` animation handling](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2670-L2700) and [collision allocation](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L3329-L3330); VSFilterMod [collision allocation](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L4531-L4532).

## unknown-tag — Unknown or misplaced override tag

The override name is not recognized, or the text escape `N`, `n`, or `h` appears inside an override block. Correct the spelling, remove the unknown tag, or move the escape into dialogue text.

References: VSFilter [command map](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1716) and [tag lookup](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2156); libass [tag dispatch](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L348) and [text escape parsing](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L1119).

## matrix-header — YCbCr matrix needs review

The matrix is missing, or a 601 matrix is selected while `PlayResY` is above 576. Renderer defaults can differ and the chosen matrix affects subtitle colors. The unsafe suggestion adds `YCbCr Matrix: None` when absent, or changes TV/PC.601 to the matching 709 range for high-resolution scripts.

References: libass documents the matrix defaults and color-mangling differences in [`ass_types.h`](https://github.com/libass/libass/blob/f61db56/libass/ass_types.h#L175) and parses the header in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L351); VSFilter parses matrix values in [`STS.cpp`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1633).

## layout-resolution — Layout resolution is missing

libass uses `LayoutResX` and `LayoutResY` for layout and scaling. If either is absent or non-positive, its fallback can depend on renderer settings. The unsafe suggestion fills missing or invalid values from the corresponding `PlayResX` and `PlayResY` values when both are valid.

References: libass parses the headers in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L888), chooses the effective layout resolution in [`ass_render.c`](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L1008), and uses it in [`ass_parse.c`](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L939).


## repeated-slash — Repeated backslash in override block

A run of backslashes before an override tag contains an extra escape character. Remove the extras while keeping the intended tag. This check is limited to override blocks; literal backslashes in dialogue text are not reported.

## style-integer — Fractional value in integer style field

Style fields such as `Bold`, `Italic`, `Alignment`, `BorderStyle`, and `MarginL/R/V` are parsed as integers. A fractional suffix is discarded by the checked libass and VSFilter parsers. The safe fix keeps the integer prefix those parsers consume.

References: libass integer conversion in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L323) and style field dispatch in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L434); VSFilter [`GetInt`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1237) and style parsing in [`STS.cpp`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1511).

## style-float — Style value exceeds VSFilter float precision

There is no universal decimal-place limit for style numbers. libass parses floating fields as doubles; VSFilter reads them into 32-bit floats, which can round values beyond roughly seven significant decimal digits. This rule only reports values where the checked conversion changes the decimal value. Its unsafe suggestion rounds to VSFilter's parsed value, which may slightly change libass output.

References: libass float parsing via [`ass_atof`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L42) and style field dispatch in [`ass.c`](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L606); VSFilter [`GetFloat`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1254) and style parsing in [`STS.cpp`](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1509).

## redundant-style — Overrides match style

This rule compares supported override slots with properties declared in the dialogue's Style. It supports Fontname, Fontsize, Bold, Italic, ScaleX, ScaleY, Spacing, Underline, StrikeOut, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Outline, Shadow, Angle, and Encoding. Colors are tracked as separate color and alpha slots; `\bord`/`\xbord`/`\ybord`, `\shad`/`\xshad`/`\yshad`, and `\fr`/`\frz` share their renderer state slots.

The safe fix removes relevant override tags only when tracked state matches the Style at every visible text boundary. Intermediate changes overwritten before text can be removed, while a later assignment that restores a Style value after visible text is kept. Other formatting tags are preserved. Style-default fallbacks after a named reset are only modeled when the active and original Style values agree: libass uses the active Style, while VSFilter retains the original defaults. An unmodeled or build-conditional operation anywhere in the dialogue blocks automatic Style fixes.
SafeFix must prove every requested primary and compatibility target; an unknown Mod capability cannot authorize edits. Relative `\fs+`/`\fs-` forms, style resets, transforms, drawing mode, malformed values, ambiguous styles, and unsupported state are skipped. Properties without a corresponding Style field, including blur, be, frx, fry, fax, fay, clip, p, and pbo, are not compared.

## repeated-brace — Extra opening brace in override block

A run such as `{{\i1}` has one extra opening brace before a matched override block. libass and VSFilter parse the block through its first `}` and look for tags after the opening brace, so the extra consecutive braces do not affect the parsed tags. The safe fix removes all but the first brace. Escaped braces such as `\{` are ignored, and unmatched runs are left alone because their rendered meaning is ambiguous.

References: libass [event override parsing](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L2066-L2075) and [tag scanning](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L282-L290); VSFilter [override block parsing](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2954-L2958) and [tag scanning](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2075-L2082).

## style-reference — Undefined style reference

A Dialogue `Style` field must resolve to a style defined in the script. Dialogue style lookup removes leading `*` characters and treats every case variation of `Default` as `Default`; other style names are case-sensitive. A named `\rStyleName` reset uses exact style-name lookup, and a bare `\r` resets to the Dialogue's base style without naming another style. Both reference forms use this warning and have no automatic fix because assx cannot determine whether to correct the name, add a style, or change the reference.

References: libass [Dialogue style lookup](https://github.com/libass/libass/blob/f61db56/libass/ass_utils.c#L253-L272) and [strict `\r` lookup](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L261-L272), [named and bare `\r` handling](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L805-L811); VSFilter [Dialogue style normalization](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1478-L1480), [unknown Dialogue style lookup](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L2222-L2250), and [named `\r` lookup](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2643-L2647).

## empty-block — Empty override block

An empty or whitespace-only override block such as `{}` or `{   }` has no rendering effect. The AST distinguishes these blocks from ordinary brace comments. The SafeFix removes the whole empty block.

Community references: ASSWipe clean level 1 and Aegisub Clean Tags both remove empty tag sections.

## junk — Junk in override block

An override block can contain raw data that is not consumed as an override tag. Pure brace comments such as `{comment}` are not reported. Mixed blocks such as `{\fs20 junk \bord2}` can report the ignored tail or raw item. This rule has no automatic fix until the consumed source range is proven equivalent across the supported renderers.

Community references include ASSWipe's junk cleanup and ASSFoundation's separate Unknown/Junk representation.

## drawing — Malformed ASS drawing

Drawing mode is parsed into commands, coordinates, and source spans. The rule reports known drawing commands with invalid coordinate arity and unknown drawing data. It does not automatically repair drawings because broken-drawing recovery has renderer-specific edge cases.

The drawing AST models `m`, `n`, `l`, `b`, `s`, `p`, and `c`, and preserves drawing state across intervening override blocks. Community references include ASSWipe drawing repair and ASSFoundation's drawing parser.

## unterminated-block — Unterminated override block

An opening brace has no closing brace after it. An escaped `\{` is not an
opening brace. Both libass and VSFilter render the remainder of the line as
literal text and apply no tag from that block. The finding points at the opening
brace and gives the column that starts the literal text.

The rule has no automatic fix. The intended end of the block is not in the
source.

References: libass [override block start](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L2063-L2066); xy-VSFilter [override block scan](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2952-L2958).

## style-colour — Malformed style colour

A Style colour field is empty or has no leading digit in its base. A `&H` or
`0x` field needs a hexadecimal digit after the prefix. A decimal field needs a
digit. The rule reports one finding per field, at the field value.

libass silently uses opaque black. xy-VSFilter rejects the line and can reload
the file with another subtitle parser. The rule is an error because the two
renderers diverge on a valid file.

There is no automatic fix. A valid replacement requires the intended colour.

References: libass [colour conversion](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L338-L342) and [integer prefix parse](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L292-L321); xy-VSFilter [colour field parse](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1237-L1252), [Style Format parse](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1553-L1557), and [style colour conversion](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L2775-L2790).

## keyword-case — Keyword case is libass-incompatible

A known keyword differs from the canonical spelling only by case. libass matches
the prefix case-sensitively. xy-VSFilter lowercases the text first, so it honors
the line while libass drops it.

A space before the colon breaks both renderers, so only a pure case difference
is reported. The checked keywords are:

- Script Info: `PlayResX:`, `PlayResY:`, `LayoutResX:`, `LayoutResY:`, `Timer:`, `WrapStyle:`, `ScaledBorderAndShadow:`, `Kerning:`, `YCbCr Matrix:`, `Language:`, `ScriptType:`.
- Styles: `Format:`, `Style:`.
- Events: `Format:`, `Dialogue:`, `Comment:`.

The finding covers the keyword and its colon. There is no automatic fix.

References: libass [Script Info keywords](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L824-L846), [Style keywords](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L880-L918), and [Event keywords](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L1015-L1047); xy-VSFilter [keyword lowercasing](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1440-L1445).

## event-format — Custom event Format line

The Events Format line names fields in an order other than the standard v4+ or
SSA order. libass maps each cell by its Format name. xy-VSFilter ignores the
Format line and reads fixed positions, so every field after the first difference
receives another value. A Format line without a Text field gets its own detail,
because libass then discards every following event line.

The rule reports one finding per Format line, at the Format value. There is no
automatic fix: a reorder changes what both renderers read.

References: libass [event field names](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L49-L58), [Format parse](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L481-L524), and [event keyword dispatch](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L995-L1010); xy-VSFilter [fixed event positions](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1455-L1472).

## short-event — Event line has fewer fields than Format declares

An event line ends before every Format field has a value. libass discards the
event. xy-VSFilter throws at the missing cell and rejects the line. The rule
reports one finding per line at the first event field, as an empty span.

There is no automatic fix. The missing values are not in the source.

References: libass [event cell count](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L455-L472) and [event parse](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L1037-L1045); xy-VSFilter [cell split](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1284-L1301) and [event line parse](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1491-L1495).

## timecode — Malformed event timecode

A Start or End cell does not have the four-group `H:MM:SS.CS` shape. The fourth
group counts centiseconds. libass reads a malformed cell as `0:00:00.00`. In
xy-VSFilter the unexpected separator shifts the later cells and the event is
discarded.

Start and End produce separate findings, at their own cells. There is no
automatic fix.

References: libass [timecode parse](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L249-L260) and [event time assignment](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L436-L438); xy-VSFilter [cell split](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1284-L1301) and [timecode parse](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1457-L1464).

## duration — Zero or negative event duration

Both timecodes parse, and End is not later than Start. With equal values the
event has zero duration and never renders. With an earlier End, libass keeps an
event that never matches a frame and xy-VSFilter drops the entry while parsing.

The rule reports at the End cell. There is no automatic fix.

References: libass [event time assignment](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L515) and [frame time selection](https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L3384-L3386); xy-VSFilter [entry time check](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L2065) and [render time range](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L2173).

## layer — Layer is not an integer

The Layer cell is empty or is not one complete integer. Both engines accept
decimal digits with an optional sign, or a `&H`/`0x` hexadecimal value. The rule
requires the whole cell, because libass stops at the first invalid character
while xy-VSFilter throws.

libass silently uses layer 0. xy-VSFilter rejects the line. The rule reports at
the Layer cell and has no automatic fix.

References: libass [integer prefix parse](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L292-L337) and [layer assignment](https://github.com/libass/libass/blob/f61db56/libass/ass.c#L516-L518); xy-VSFilter [cell split](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1284-L1305) and [Layer read](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1456).

## effect — Effect field is not honored

An empty Effect field is correct. Only `Banner;`, `Scroll up;`, and `Scroll down;`
change rendering, and each needs integer parameters separated by `;`. `Banner;`
needs one parameter and each scroll needs three.

One condition covers three cases:

- The name is not one of the three, so every renderer treats the field as a no-op.
- The name differs in case. libass matches case-sensitively, while both VSFilter forks compare case-insensitively.
- A recognized name has too few parameters, so the effect has no effect.

The finding covers the Effect cell. There is no automatic fix.

References: libass [effect parse](https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L923-L997); xy-VSFilter [effect parse](https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1940-L1988); VSFilterMod [effect parse](https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2113-L2164).

## karaoke — Karaoke timing exceeds the event

Renderer-scoped validation reads the `karaoke_cursor` state from the same
semantic observer that produces the other dialogue findings. It also requires
source-verified signatures for every contributing karaoke tag. Partially
consumed arguments, such as `\k200junk`, and inferred-only VSFilterMod
signatures cannot produce a definitive timing warning. Standalone event analysis
uses the same evaluator and proof checks, including the default CLI.

The rule sums the `\k`, `\K`, `\kf`, and `\ko` durations. It reports a sum
that runs past the event end, where the last syllables never reach their
highlight window. There is no automatic fix.

## unresolved — Renderer command interpretation is unresolved

Explicit renderer-scoped validation uses this warning when a command is disabled or build-conditional, matched but semantically unmodeled, backed only by inferred signature evidence, or malformed in a way that prevents a verified interpretation. The finding does not assert a cross-renderer incompatibility and never authorizes an automatic edit. The default CLI also emits this warning for unresolved or ignored cross-renderer compatibility observations. Unverified tracked-state agreement remains one of these observations. See [renderer-scoped lint validation](../packages/lint.md) and [compatibility](../packages/compatibility.md).

## alignment — Style Alignment is outside 1 to 9

The `Alignment` field of an ASS v4+ Style row is a numpad value. The pinned renderers place text for values 1 to 9 only. libass maps the value with `numpad2align` at parse; xy-VSFilter and VSFilterMod keep the raw integer and derive the position at use. This rule reports a whole-integer value outside 1 to 9. A fractional value stays an style-integer finding, and a malformed value stays an style-colour or style-integer finding, so the rules do not overlap.

Legacy SSA `Alignment` uses a bitfield mapping and is not checked by this rule. The rule has no automatic fix because the intended alignment is unknown. See [Style field conversion evidence](../packages/compatibility.md#style-field-conversion-evidence).

## border-box — BorderStyle box scaling differs

This warning reports a specific box-scaling difference in an ASS v4+ Style definition.
It requires `BorderStyle=3`, positive `ScaleX` and `ScaleY`, and positive `Outline`.
At least one scale must differ from 100.
The selected profiles must include VSFilterMod and either libass or xy-VSFilter.
libass and xy-VSFilter apply extra font scaling to box dimensions.
VSFilterMod skips that extra scaling for its box polygon, without a build guard.

The comparison requires matching resolved fields and complete Style rows.
An unrelated layout difference does not hide this warning.
It accepts whole decimal BorderStyle values and finite decimal scale and outline values.
Scale and outline values must have the same float32 and float64 value.
Missing, repeated, malformed, and precision-dependent fields do not establish this condition.
Ordinary boxes at 100 percent do not produce this warning.
Zero-outline boxes and shadow-only boxes are outside this check.

This warning describes the declared Style, including an unused Style.
It does not trace Dialogue overrides or predict final pixels.
Check box dimensions in each selected renderer.
There is no automatic fix. See [the source evidence](ass-style-fields.xml).

## border-mode — BorderStyle rendering mode differs

This warning reports `BorderStyle=4` in an ASS v4+ Style definition.
It requires libass and at least one VSFilter renderer in the selected profiles.
libass selects a background around the complete event.
xy-VSFilter and VSFilterMod select a normal glyph outline.
A comparison between the two VSFilter renderers does not produce this warning.

Both profiles must read BorderStyle from the same source field in complete Style rows.
The value must be a whole decimal integer.
SSA, v4++, malformed values, hexadecimal values, and mismatched BorderStyle fields are outside this check.
It describes the declared Style, including an unused Style.
Dialogue overrides, named resets, and host settings can change the final output.
Review the intended mode in each selected renderer.
There is no automatic fix. See [the source evidence](ass-style-fields.xml).

## Fixes

Severity and fix safety are independent metadata: a diagnostic can have a `SafeFix` or an `UnsafeFix`.

A `SafeFix` preserves the rendered output in the supported renderer scope. The checker applies these edits with `--fix`. An `UnsafeFix` may change output and requires explicit opt-in with `--unsafe-fix`; that option also applies available safe fixes. Without either option, assx only reports findings. The command ends with counts of safe, unsafe, and non-auto-fixable findings. Text output keeps individual messages short; JSON retains detailed diagnostics and edit ranges.

The two header rules are unsafe: adding or changing `YCbCr Matrix` can change colors, and setting `LayoutResX/Y` can change libass scaling and tag behavior.

Formatting is not implemented as a separate command yet. Preserve tag order by default: precedence and interactions, especially inside `\t(...)`, can make reordering change output. A future formatter should only normalize syntax when it can prove the rendered result is unchanged.

assx is not a complete ASS validator or an exact renderer emulator. Citations identify the source basis for modeled rules, not a guarantee for every malformed input.

