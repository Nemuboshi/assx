package spec

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

// SemanticKind selects specialized behavior only when declarative assignment
// policy is insufficient. Zero means generic, allocation-free evaluation.
type SemanticKind uint8

const (
	SemanticGeneric SemanticKind = iota
	SemanticTransform
	SemanticClip
	SemanticStyleReset
	SemanticKaraoke
	SemanticDrawingMode
	SemanticFontSize
)

type TagSpec struct {
	Semantic            SemanticKind
	TransformComparable bool
	Slots               []string
	Behavior            Behavior
	Value               ValueKind
	Min                 int
	Max                 int
	Counts              []int
	VSFilterModOnly     bool
}

// TagSpecs describes recognized override names and their renderer-facing semantics.
// clip/iclip choose their final slot behavior by argument shape in the semantic layer.
var TagSpecs = map[string]TagSpec{
	"b": {Slots: []string{"bold"}, Value: BoldValue}, "i": {Slots: []string{"italic"}, Value: IntegerValue, Min: 0, Max: 1},
	"u": {Slots: []string{"underline"}, Value: IntegerValue, Min: 0, Max: 1}, "s": {Slots: []string{"strikeout"}, Value: IntegerValue, Min: 0, Max: 1},
	"fn": {Slots: []string{"fontname"}, Value: FontNameValue}, "fe": {Slots: []string{"charset"}, Value: NoValue},
	"fs": {TransformComparable: true, Semantic: SemanticFontSize, Slots: []string{"fontsize"}, Value: NumberValue}, "fscx": {TransformComparable: true, Slots: []string{"scale_x"}, Value: NumberValue},
	"fscy": {TransformComparable: true, Slots: []string{"scale_y"}, Value: NumberValue}, "fsc": {Slots: []string{"scale_x", "scale_y"}, Value: NoValue},
	"fsp": {TransformComparable: true, Slots: []string{"spacing"}, Value: NumberValue}, "frx": {TransformComparable: true, Slots: []string{"frx"}, Value: NumberValue},
	"fry": {TransformComparable: true, Slots: []string{"fry"}, Value: NumberValue}, "frz": {TransformComparable: true, Slots: []string{"frz"}, Value: NumberValue}, "fr": {TransformComparable: true, Slots: []string{"frz"}, Value: NumberValue},
	"fax": {TransformComparable: true, Slots: []string{"fax"}, Value: NumberValue}, "fay": {TransformComparable: true, Slots: []string{"fay"}, Value: NumberValue},
	"xbord": {TransformComparable: true, Slots: []string{"border_x"}, Value: NumberValue}, "ybord": {TransformComparable: true, Slots: []string{"border_y"}, Value: NumberValue},
	"bord": {TransformComparable: true, Slots: []string{"border_x", "border_y"}, Value: NumberValue}, "xshad": {TransformComparable: true, Slots: []string{"shadow_x"}, Value: NumberValue},
	"yshad": {TransformComparable: true, Slots: []string{"shadow_y"}, Value: NumberValue}, "shad": {TransformComparable: true, Slots: []string{"shadow_x", "shadow_y"}, Value: NumberValue},
	"be": {TransformComparable: true, Slots: []string{"be"}, Value: NumberValue}, "blur": {TransformComparable: true, Slots: []string{"blur"}, Value: NumberValue},
	"q": {Slots: []string{"wrap_style"}, Value: IntegerValue, Min: 0, Max: 3}, "p": {Semantic: SemanticDrawingMode, Slots: []string{"drawing_scale"}, Value: IntegerValue},
	"pbo": {Slots: []string{"pbo"}, Value: NumberValue}, "c": {TransformComparable: true, Slots: []string{"c1"}, Value: HexValue},
	"1c": {TransformComparable: true, Slots: []string{"c1"}, Value: HexValue}, "2c": {TransformComparable: true, Slots: []string{"c2"}, Value: HexValue},
	"3c": {TransformComparable: true, Slots: []string{"c3"}, Value: HexValue}, "4c": {TransformComparable: true, Slots: []string{"c4"}, Value: HexValue},
	"1a": {TransformComparable: true, Slots: []string{"a1"}, Value: HexValue}, "2a": {TransformComparable: true, Slots: []string{"a2"}, Value: HexValue},
	"3a": {TransformComparable: true, Slots: []string{"a3"}, Value: HexValue}, "4a": {TransformComparable: true, Slots: []string{"a4"}, Value: HexValue},
	"alpha": {TransformComparable: true, Slots: []string{"a1", "a2", "a3", "a4"}, Value: HexValue},
	"an":    {Slots: []string{"alignment"}, Behavior: FirstWins, Value: IntegerValue, Min: 1, Max: 9},
	"a":     {Slots: []string{"alignment"}, Behavior: FirstWins, Value: IntegerValue, Min: 1, Max: 11},
	"pos":   {Slots: []string{"position"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2}},
	"move":  {Slots: []string{"position"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{4, 6}},
	"org":   {Slots: []string{"origin"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2}},
	"fade":  {Slots: []string{"fade"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2, 7}},
	"fad":   {Slots: []string{"fade"}, Behavior: FirstWins, Value: NumberListValue, Counts: []int{2, 7}},
	"clip":  {TransformComparable: true, Semantic: SemanticClip, Value: RectValue, Counts: []int{1, 2, 4}}, "iclip": {TransformComparable: true, Semantic: SemanticClip, Value: RectValue, Counts: []int{1, 2, 4}},
	"r": {Semantic: SemanticStyleReset, Behavior: StyleReset}, "t": {Semantic: SemanticTransform, Behavior: Transition}, "k": {Semantic: SemanticKaraoke, Behavior: Accumulate}, "K": {Semantic: SemanticKaraoke, Behavior: Accumulate},
	"kf": {Semantic: SemanticKaraoke, Behavior: Accumulate}, "ko": {Semantic: SemanticKaraoke, Behavior: Accumulate}, "kt": {Semantic: SemanticKaraoke, Behavior: Accumulate},
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

var KeepOnStyleReset = map[string]bool{
	"wrap_style":     true,
	"drawing_scale":  true,
	"karaoke_cursor": true,
	"pbo":            true,
	"clip_rect":      true,
}
