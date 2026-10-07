package lint

const (
	IssueArgumentCount          = "ASS001"
	IssueInvalidValue           = "ASS002"
	IssueRendererDiff           = "ASS003"
	IssueFontComma              = "ASS004"
	IssueVSFilterModTag         = "ASS005"
	IssueNoEffect               = "ASS006"
	IssueUnknownTag             = "ASS007"
	IssueMatrixHeader           = "ASS008"
	IssueLayoutRes              = "ASS009"
	IssueRepeatedSlash          = "ASS010"
	IssueStyleInteger           = "ASS011"
	IssueStyleFloat             = "ASS012"
	IssueRedundantFontOverrides = "ASS013"
	IssueRepeatedOpenBrace      = "ASS014"
	IssueFontMissing            = "ASS015"
	IssueMissingGlyphs          = "ASS016"
	IssueUndefinedStyle         = "ASS017"
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
		Sources: []string{"https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L231", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L425", "https://github.com/libass/libass/blob/f61db56/libass/ass_parse.c#L749", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2205", "https://github.com/Masaiki/xy-VSFilter/blob/135a3015/src/subtitles/RTS.cpp#L2315"},
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
	IssueRedundantFontOverrides: {
		ID: IssueRedundantFontOverrides, Severity: Suggestion, FixSafety: SafeFix, Title: "Overrides match style",
		Description: "Style-backed override tags leave the corresponding Style properties unchanged across dialogue text.",
		Fix:         "Remove the redundant font override tags.",
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
}

// Behavior describes how a tag competes for its state slot. The zero value is Assign.
type Behavior uint8

const (
	Assign Behavior = iota
	FirstWins
	Accumulate
	Transition
	StyleReset
)

type ValueKind uint8

const (
	NoValue ValueKind = iota
	IntegerValue
	NumberValue
	HexValue
	BoldValue
	FontNameValue
	NumberListValue
	RectValue
)

type TagSpec struct {
	Slots           []string
	Behavior        Behavior // Omitted means Assign; non-assign rules are explicit.
	Value           ValueKind
	Min             int
	Max             int
	Counts          []int
	VSFilterModOnly bool
}

// TagSpecs is the registry for recognized override names and behavior.
// VSFilterModOnly tags are recognized so they can be warned about, but their state is not modeled.
// clip/iclip choose behavior by shape: rectangular clips assign, vector clips are first-wins.
var TagSpecs = map[string]TagSpec{
	"b": {Slots: []string{"bold"}, Value: BoldValue}, "i": {Slots: []string{"italic"}, Value: IntegerValue, Min: 0, Max: 1},
	"u": {Slots: []string{"underline"}, Value: IntegerValue, Min: 0, Max: 1}, "s": {Slots: []string{"strikeout"}, Value: IntegerValue, Min: 0, Max: 1},
	"fn": {Slots: []string{"fontname"}, Value: FontNameValue}, "fe": {Slots: []string{"charset"}, Value: NoValue},
	"fs": {Slots: []string{"fontsize"}, Value: NumberValue}, "fscx": {Slots: []string{"scale_x"}, Value: NumberValue},
	"fscy": {Slots: []string{"scale_y"}, Value: NumberValue}, "fsc": {Slots: []string{"scale_x", "scale_y"}, Value: NoValue},
	"fsp": {Slots: []string{"spacing"}, Value: NumberValue}, "frx": {Slots: []string{"frx"}, Value: NumberValue},
	"fry": {Slots: []string{"fry"}, Value: NumberValue}, "frz": {Slots: []string{"frz"}, Value: NumberValue}, "fr": {Slots: []string{"frz"}, Value: NumberValue},
	"fax": {Slots: []string{"fax"}, Value: NumberValue}, "fay": {Slots: []string{"fay"}, Value: NumberValue},
	"xbord": {Slots: []string{"border_x"}, Value: NumberValue}, "ybord": {Slots: []string{"border_y"}, Value: NumberValue},
	"bord": {Slots: []string{"border_x", "border_y"}, Value: NumberValue}, "xshad": {Slots: []string{"shadow_x"}, Value: NumberValue},
	"yshad": {Slots: []string{"shadow_y"}, Value: NumberValue}, "shad": {Slots: []string{"shadow_x", "shadow_y"}, Value: NumberValue},
	"be": {Slots: []string{"be"}, Value: NumberValue}, "blur": {Slots: []string{"blur"}, Value: NumberValue},
	"q": {Slots: []string{"wrap_style"}, Value: IntegerValue, Min: 0, Max: 3}, "p": {Slots: []string{"drawing_scale"}, Value: IntegerValue},
	"pbo": {Slots: []string{"pbo"}, Value: NumberValue}, "c": {Slots: []string{"c1"}, Value: HexValue},
	"1c": {Slots: []string{"c1"}, Value: HexValue}, "2c": {Slots: []string{"c2"}, Value: HexValue},
	"3c": {Slots: []string{"c3"}, Value: HexValue}, "4c": {Slots: []string{"c4"}, Value: HexValue},
	"1a": {Slots: []string{"a1"}, Value: HexValue}, "2a": {Slots: []string{"a2"}, Value: HexValue},
	"3a": {Slots: []string{"a3"}, Value: HexValue}, "4a": {Slots: []string{"a4"}, Value: HexValue},
	"alpha": {Slots: []string{"a1", "a2", "a3", "a4"}, Value: HexValue},
	"an":    {Slots: []string{"alignment"}, Behavior: FirstWins, Value: IntegerValue, Min: 1, Max: 9},
	"a":     {Slots: []string{"alignment"}, Behavior: FirstWins, Value: IntegerValue, Min: 1, Max: 11},
	"pos":   {Slots: []string{"position"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2}},
	"move":  {Slots: []string{"position"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{4, 6}},
	"org":   {Slots: []string{"origin"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2}},
	"fade":  {Slots: []string{"fade"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2, 7}},
	"fad":   {Slots: []string{"fade"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2, 7}},
	"clip":  {Value: RectValue, Counts: []int{1, 2, 4}}, "iclip": {Value: RectValue, Counts: []int{1, 2, 4}},
	"r": {Behavior: StyleReset}, "t": {Behavior: Transition}, "k": {Behavior: Accumulate}, "K": {Behavior: Accumulate},
	"kf": {Behavior: Accumulate}, "ko": {Behavior: Accumulate}, "kt": {Behavior: Accumulate},
	"N": {}, "n": {}, "h": {},
	"1img": {VSFilterModOnly: true}, "2img": {VSFilterModOnly: true}, "3img": {VSFilterModOnly: true}, "4img": {VSFilterModOnly: true},
	"1vc": {VSFilterModOnly: true}, "2vc": {VSFilterModOnly: true}, "3vc": {VSFilterModOnly: true}, "4vc": {VSFilterModOnly: true},
	"1va": {VSFilterModOnly: true}, "2va": {VSFilterModOnly: true}, "3va": {VSFilterModOnly: true}, "4va": {VSFilterModOnly: true},
	"distort": {VSFilterModOnly: true}, "frs": {VSFilterModOnly: true}, "fsvp": {VSFilterModOnly: true}, "fshp": {VSFilterModOnly: true},
	"jitter": {VSFilterModOnly: true}, "lua": {VSFilterModOnly: true}, "mover": {VSFilterModOnly: true}, "moves3": {VSFilterModOnly: true},
	"moves4": {VSFilterModOnly: true}, "movevc": {VSFilterModOnly: true}, "rnds": {VSFilterModOnly: true}, "rndx": {VSFilterModOnly: true},
	"rndy": {VSFilterModOnly: true}, "rndz": {VSFilterModOnly: true}, "rnd": {VSFilterModOnly: true}, "xblur": {VSFilterModOnly: true},
	"yblur": {VSFilterModOnly: true}, "z": {VSFilterModOnly: true}, "ortho": {VSFilterModOnly: true}, "blend": {VSFilterModOnly: true},
}

var KeepOnStyleReset = map[string]bool{"wrap_style": true, "drawing_scale": true, "pbo": true, "clip_rect": true}
