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

type TagSpec struct {
	Slots           []string
	Behavior        Behavior
	Value           ValueKind
	Min             int
	Max             int
	Counts          []int
	VSFilterModOnly bool
}

// TagSpecs describes recognized override names and their renderer-facing semantics.
// clip/iclip choose their final slot behavior by argument shape in the semantic layer.
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

var KeepOnStyleReset = map[string]bool{
	"wrap_style":    true,
	"drawing_scale": true,
	"pbo":           true,
	"clip_rect":     true,
}
