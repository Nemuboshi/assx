package ass

import "strings"

type Tag struct {
	Name            string   `json:"tag"`
	Args            []string `json:"arguments,omitempty"`
	Column          int      `json:"column"`
	Paren           bool     `json:"-"`
	Start           int      `json:"-"`
	End             int      `json:"-"`
	InTransition    bool     `json:"-"`
	RepeatedSlashes int      `json:"-"`
	SlashStart      int      `json:"-"`
	Children        []Tag    `json:"-"`
	Raw             string   `json:"-"`
}

// RawArgument returns the argument text as written, including whitespace that
// can change a renderer's reset behavior.
func (tag Tag) RawArgument() (string, bool) {
	if len(tag.Args) > 1 {
		return "", false
	}
	if tag.Raw == "" {
		if len(tag.Args) == 0 {
			return "", true
		}
		return tag.Args[0], true
	}
	raw := strings.TrimLeft(strings.TrimPrefix(tag.Raw, "\\"), " \t")
	if !strings.HasPrefix(raw, tag.Name) {
		return "", false
	}
	raw = raw[len(tag.Name):]
	if tag.Paren {
		if len(raw) < 2 || raw[0] != '(' || raw[len(raw)-1] != ')' {
			return "", false
		}
		raw = raw[1 : len(raw)-1]
	}
	return raw, true
}

// IntegerArgument reads the leading signed decimal integer consumed by renderers.
// Overflow is unknown because libass and Windows integer conversion differ.
func (tag Tag) IntegerArgument() (int32, bool) {
	raw, known := tag.RawArgument()
	if !known {
		return 0, false
	}
	value := DecodeRendererInteger(raw)
	return int32(value.Integer), value.Status == ValueValid
}
