package edit

import (
	"fmt"
	"sort"
	"strings"
)

type TextEdit struct {
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Replacement string `json:"replacement"`
}

func Apply(text string, edits []TextEdit) (string, error) {
	if len(edits) == 0 {
		return text, nil
	}

	ordered := append([]TextEdit(nil), edits...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Start == ordered[j].Start {
			return ordered[i].End < ordered[j].End
		}
		return ordered[i].Start < ordered[j].Start
	})

	var out strings.Builder
	cursor := 0
	for _, edit := range ordered {
		if edit.Start < cursor || edit.Start < 0 || edit.End < edit.Start || edit.End > len(text) {
			return "", fmt.Errorf("overlapping or invalid edit range [%d:%d]", edit.Start, edit.End)
		}
		out.WriteString(text[cursor:edit.Start])
		out.WriteString(edit.Replacement)
		cursor = edit.End
	}
	out.WriteString(text[cursor:])
	return out.String(), nil
}
