package lint

const (
	IssueArgumentCount           = "ASS001"
	IssueInvalidValue            = "ASS002"
	IssueRendererDiff            = "ASS003"
	IssueFontComma               = "ASS004"
	IssueVSFilterModTag          = "ASS005"
	IssueNoEffect                = "ASS006"
	IssueUnknownTag              = "ASS007"
	IssueMatrixHeader            = "ASS008"
	IssueLayoutRes               = "ASS009"
	IssueRepeatedSlash           = "ASS010"
	IssueStyleInteger            = "ASS011"
	IssueStyleFloat              = "ASS012"
	IssueRedundantStyleOverrides = "ASS013"
	IssueRepeatedOpenBrace       = "ASS014"
	IssueFontMissing             = "ASS015"
	IssueMissingGlyphs           = "ASS016"
	IssueUndefinedStyle          = "ASS017"
	IssueEmptyOverrideBlock      = "ASS018"
	IssueOverrideJunk            = "ASS019"
	IssueMalformedDrawing        = "ASS020"
	IssueUnterminatedBlock       = "ASS021"
	IssueMalformedStyleColour    = "ASS022"
	IssueKeywordCase             = "ASS023"
	IssueEventFormat             = "ASS024"
	IssueShortEvent              = "ASS025"
	IssueTimecode                = "ASS026"
	IssueEventDuration           = "ASS027"
	IssueEventLayer              = "ASS028"
	IssueEffectField             = "ASS029"
	IssueKaraoke                 = "ASS030"
	IssueRendererUnresolved      = "ASS031"
)

type Severity string

const (
	Error      Severity = "error"
	Warning    Severity = "warning"
	Suggestion Severity = "suggestion"
)

type Rule struct {
	ID          string    `json:"id"`
	Severity    Severity  `json:"severity"`
	FixSafety   FixSafety `json:"fix_safety,omitempty"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Fix         string    `json:"suggested_fix"`
	Sources     []string  `json:"sources"`
}

type FixSafety string

const (
	SafeFix   FixSafety = "safe"
	UnsafeFix FixSafety = "unsafe"
)

var Rules = map[string]Rule{
	IssueRendererUnresolved: {
		ID: IssueRendererUnresolved, Severity: Warning, Title: "Renderer command interpretation is unresolved",
		Description: "The selected renderer rejects, ignores, or cannot conclusively interpret this invocation.",
		Fix:         "Check the exact command and target build before changing the source.",
	},
	IssueArgumentCount: {
		ID: IssueArgumentCount, Severity: Error, Title: "Invalid argument count",
		Description: "A tag has an argument count outside the checked forms.", Fix: "Correct the tag's argument list.",
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L405", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L494", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L608", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L625", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L730", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2352", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2396", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2575", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2615"},
	},
	IssueInvalidValue: {
		ID: IssueInvalidValue, Severity: Error, Title: "Invalid tag value",
		Description: "A checked tag value is malformed or outside its supported integer range.", Fix: "Replace it with a syntactically valid value in the accepted range.",
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L584", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L825", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L882", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2301", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2344", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2635"},
	},
	IssueRendererDiff: {
		ID: IssueRendererDiff, Severity: Warning, Title: "Renderer behavior differs",
		Description: "This tag form can be parsed or rendered differently by libass and VSFilter.", Fix: "Choose an unambiguous form or verify the intended output in both renderers.",
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L231", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L404-L423", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L425", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L595-L604", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L729-L744", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L749", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2205", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2308-L2313", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2315", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2352-L2382", "https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2909-L2914", "https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2953-L2998"},
	},
	IssueFontComma: {
		ID: IssueFontComma, Severity: Warning, Title: "Comma splits font name",
		Description: "Commas in parenthesized font names split the tag arguments.", Fix: "Use concatenated syntax for a font name containing commas.",
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L302", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2110", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2447"},
	},
	IssueVSFilterModTag: {
		ID: IssueVSFilterModTag, Severity: Warning, Title: "VSFilterMod tag may be unsupported by libass/VSFilter",
		Description: "This VSFilterMod-only tag may not be supported by libass or original VSFilter. Content using it cannot be safely auto-fixed.",
		Fix:         "Verify support and rendering in the target renderer; no safe automatic fix is available for content using this tag.",
		Sources:     []string{"https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2745-L3717", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1716-L1769", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L281-L888"},
	},
	IssueNoEffect: {
		ID: IssueNoEffect, Severity: Suggestion, FixSafety: SafeFix, Title: "Override has no effect",
		Description: "An override is replaced, reset, or ignored before it affects dialogue text, or repeats the value already active in every affected state slot.", Fix: "Remove it or move it to the intended text boundary.",
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L584", "https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L1075", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2301", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2396", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2575", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2643"},
	},
	IssueUnknownTag: {
		ID: IssueUnknownTag, Severity: Suggestion, Title: "Unknown or misplaced override tag",
		Description: "The tag is unknown, or a text escape was placed in an override block.", Fix: "Correct the spelling, remove the tag, or move the text escape into dialogue text.",
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L348", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L1119", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1716", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2156"},
	},
	IssueMatrixHeader: {
		ID: IssueMatrixHeader, Severity: Suggestion, FixSafety: UnsafeFix, Title: "YCbCr matrix needs review",
		Description: "An absent YCbCr matrix defaults to renderer-dependent color conversion, or TV/PC.601 is used above PlayResY 576.",
		Fix:         "Set an explicit matrix. The suggested edit uses None when absent and the matching 709 range when 601 is used above 576p.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_types.h#L175", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L351", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1633"},
	},
	IssueLayoutRes: {
		ID: IssueLayoutRes, Severity: Suggestion, FixSafety: UnsafeFix, Title: "Layout resolution is missing",
		Description: "libass uses layout resolution to scale font, blur, borders, shadows, and layout calculations.",
		Fix:         "Set missing LayoutResX and LayoutResY values to the corresponding PlayRes dimensions after reviewing the rendering change.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L888", "https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L1008", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L939"},
	},
	IssueRepeatedSlash: {
		ID: IssueRepeatedSlash, Severity: Suggestion, FixSafety: SafeFix, Title: "Repeated backslash in override block",
		Description: "More than one backslash precedes an override tag.", Fix: "Remove the extra backslash and keep the intended tag.",
	},
	IssueStyleInteger: {
		ID: IssueStyleInteger, Severity: Suggestion, FixSafety: SafeFix, Title: "Fractional value in integer style field",
		Description: "This style field is parsed as an integer; the fractional part is ignored.", Fix: "Remove the fractional part while keeping the renderer-consumed integer value.",
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L323", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L434", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1237", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1511"},
	},
	IssueStyleFloat: {
		ID: IssueStyleFloat, Severity: Suggestion, FixSafety: UnsafeFix, Title: "Style value exceeds VSFilter float precision",
		Description: "This field is parsed as a double by libass and a 32-bit float by VSFilter; significant digits can be rounded differently.",
		Fix:         "Round to the VSFilter 32-bit float value only after reviewing the small rendering change in libass.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L435", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1254", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1509"},
	},
	IssueRedundantStyleOverrides: {
		ID: IssueRedundantStyleOverrides, Severity: Suggestion, FixSafety: SafeFix, Title: "Overrides match style",
		Description: "Style-backed override tags leave the corresponding Style properties unchanged across dialogue text.",
		Fix:         "Remove the redundant override tags.",
	},
	IssueRepeatedOpenBrace: {
		ID: IssueRepeatedOpenBrace, Severity: Suggestion, FixSafety: SafeFix, Title: "Extra opening brace in override block",
		Description: "Only the first of consecutive opening braces starts the override block; the extras are ignored while tags are parsed.",
		Fix:         "Remove the extra opening brace(s).",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L2066-L2075", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L282-L290", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2954-L2958", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2075-L2082"},
	},
	IssueFontMissing: {
		ID: IssueFontMissing, Severity: Suggestion, Title: "Subtitle font is missing",
		Description: "The font family used by subtitle text was not found in the selected font set.",
		Fix:         "Install the font or point --font-dir at a folder containing it.",
	},
	IssueMissingGlyphs: {
		ID: IssueMissingGlyphs, Severity: Suggestion, Title: "Font is missing subtitle characters",
		Description: "The selected font does not contain glyphs for some subtitle characters.",
		Fix:         "Use a font that contains the missing characters.",
	},
	IssueUndefinedStyle: {
		ID: IssueUndefinedStyle, Severity: Warning, Title: "Undefined style reference",
		Description: "A dialogue or override tag references a style that is not defined in the script.",
		Fix:         "Correct the style name or define the referenced style.",
	},
	IssueEmptyOverrideBlock: {
		ID: IssueEmptyOverrideBlock, Severity: Suggestion, FixSafety: SafeFix, Title: "Empty override block",
		Description: "An empty or whitespace-only override block has no effect on rendering.",
		Fix:         "Remove the empty override block.",
		Sources:     []string{"https://github.com/TypesettingTools/line0-Aegisub-Scripts/blob/master/l0.ASSWipe.moon#L45-L55", "https://github.com/TypesettingTools/Aegisub/blob/master/automation/include/cleantags.lua#L105-L106"},
	},
	IssueOverrideJunk: {
		ID: IssueOverrideJunk, Severity: Suggestion, Title: "Junk in override block",
		Description: "An override block contains non-tag data that is ignored while override tags are parsed.",
		Fix:         "Review and remove the ignored data.",
		Sources:     []string{"https://github.com/TypesettingTools/line0-Aegisub-Scripts/blob/master/l0.ASSWipe.moon#L53", "https://github.com/TypesettingTools/ASSFoundation/blob/master/l0/ASSFoundation/FoundationMethods.moon#L171-L176"},
	},
	IssueMalformedDrawing: {
		ID: IssueMalformedDrawing, Severity: Warning, Title: "Malformed ASS drawing",
		Description: "A drawing contains a command with missing, extra, or malformed coordinates.",
		Fix:         "Correct the drawing command after reviewing renderer behavior.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_drawing.c#L160-L240", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L789-L895", "https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L869-L970"},
	},
	IssueUnterminatedBlock: {
		ID: IssueUnterminatedBlock, Severity: Warning, Title: "Unterminated override block",
		Description: "An opening brace has no closing brace, so libass and VSFilter render the remainder of the line as literal text and apply no tags from it.",
		Fix:         "Add the closing brace or remove the opening brace.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L2063-L2066", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2952-L2958"},
	},
	IssueMalformedStyleColour: {
		ID: IssueMalformedStyleColour, Severity: Error, Title: "Malformed style colour",
		Description: "A Style colour field has no valid leading digit for its base. libass silently uses opaque black, while VSFilter rejects the line and may reload the file with another subtitle parser.",
		Fix:         "Write the colour as &HAABBGGRR& hexadecimal digits.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L338-L342", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L292-L321", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1237-L1252", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1553-L1557", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L2775-L2790"},
	},
	IssueKeywordCase: {
		ID: IssueKeywordCase, Severity: Warning, Title: "Keyword case is libass-incompatible",
		Description: "The line keyword is not in the form libass matches case-sensitively. VSFilter lowercases keywords first, so the line may be honored there while libass silently ignores it.",
		Fix:         "Use the canonical keyword spelling (for example Dialogue:, Style:, Format:, PlayResX:).",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L824-L846", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L880-L918", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L1015-L1047", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1440-L1445"},
	},
	IssueEventFormat: {
		ID: IssueEventFormat, Severity: Warning, Title: "Custom event Format line",
		Description: "The Events Format line is not the standard v4+ or SSA order. libass reads event fields by name from this line while VSFilter ignores it and uses fixed positions, so fields beyond the first difference are assigned differently.",
		Fix:         "Use the standard order, or verify each field in both renderers.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L49-L58", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L481-L524", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L995-L1010", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1455-L1472"},
	},
	IssueShortEvent: {
		ID: IssueShortEvent, Severity: Error, Title: "Event line has fewer fields than Format declares",
		Description: "A Dialogue line ends before every Format field has a value. libass discards the event silently, and VSFilter throws at the missing cell and rejects the line.",
		Fix:         "Add the missing fields so the line has a value for every Format name.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L455-L472", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L1037-L1045", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1284-L1301", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1491-L1495"},
	},
	IssueTimecode: {
		ID: IssueTimecode, Severity: Error, Title: "Malformed event timecode",
		Description: "A Start or End field is not H:MM:SS.CS. libass reads such a time as 0:00:00.00, and VSFilter stops at the unexpected separator, desynchronizing later fields and discarding the event.",
		Fix:         "Write the timecode with three colon-separated groups and a two-digit centisecond part after the dot.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L249-L260", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L436-L438", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1284-L1301", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1457-L1464"},
	},
	IssueEventDuration: {
		ID: IssueEventDuration, Severity: Warning, Title: "Zero or negative event duration",
		Description: "End is not later than Start. libass keeps an event that never matches a frame, and VSFilter drops entries whose start is after their end while parsing.",
		Fix:         "Set End later than Start.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L515", "https://github.com/libass/libass/blob/f61db56/libass/ass_render.c#L3384-L3386", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L2065", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L2173"},
	},
	IssueEventLayer: {
		ID: IssueEventLayer, Severity: Error, Title: "Layer is not an integer",
		Description: "The Layer field has no valid leading integer. libass silently uses layer 0, and VSFilter throws in NextInt and rejects the whole line.",
		Fix:         "Write Layer as a plain decimal or &H/0x hexadecimal integer.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass.c#L292-L337", "https://github.com/libass/libass/blob/f61db56/libass/ass.c#L516-L518", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1284-L1305", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/STS.cpp#L1456"},
	},
	IssueEffectField: {
		ID: IssueEffectField, Severity: Warning, Title: "Effect field is not honored",
		Description: "Only Banner;, Scroll up;, and Scroll down; (followed by their parameters) change rendering. libass matches these prefixes case-sensitively while VSFilter compares case-insensitively, and any other content is a silent no-op.",
		Fix:         "Use the canonical effect spelling with the parameters it requires, or leave the field empty.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L923-L997", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L1940-L1988", "https://github.com/AmusementClub/VSFilterMod/blob/7a00567e4a49b6310691b9a6791646b2a018bfa2/src/subtitles/RTS.cpp#L2113-L2164"},
	},
	IssueKaraoke: {
		ID: IssueKaraoke, Severity: Warning, Title: "Karaoke timing exceeds the event",
		Description: "The summed \\k, \\K, \\kf, and \\ko durations run past the event end, so the last syllables never reach their highlight window.",
		Fix:         "Reduce syllable durations, rebalance them across the line, or extend the event duration.",
		Sources:     []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L845-L868", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L1025-L1080", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2537-L2573", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2944-L2945"},
	},
}
