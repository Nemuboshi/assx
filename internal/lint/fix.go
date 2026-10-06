package lint

import (
	"fmt"
	"sort"
	"strings"
)

func CountFixes(diagnostics []Diagnostic) (safe, unsafe, unfixable int) {
	for _, diagnostic := range diagnostics {
		if len(diagnostic.Edits) == 0 {
			unfixable++
			continue
		}
		switch diagnostic.FixSafety {
		case SafeFix:
			safe++
		case UnsafeFix:
			unsafe++
		default:
			unfixable++
		}
	}
	return safe, unsafe, unfixable
}
func ApplyFixes(text string, diagnostics []Diagnostic, includeUnsafe bool) (string, int, error) {
	var edits []TextEdit
	fixes := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety != SafeFix && !(includeUnsafe && diagnostic.FixSafety == UnsafeFix) {
			continue
		}
		if len(diagnostic.Edits) == 0 {
			continue
		}
		edits = append(edits, diagnostic.Edits...)
		fixes++
	}
	if len(edits) == 0 {
		return text, 0, nil
	}
	sort.SliceStable(edits, func(i, j int) bool {
		if edits[i].Start == edits[j].Start {
			return edits[i].End < edits[j].End
		}
		return edits[i].Start < edits[j].Start
	})
	var out strings.Builder
	cursor := 0
	for _, edit := range edits {
		if edit.Start < cursor || edit.Start < 0 || edit.End < edit.Start || edit.End > len(text) {
			return "", 0, fmt.Errorf("overlapping or invalid fix range [%d:%d]", edit.Start, edit.End)
		}
		out.WriteString(text[cursor:edit.Start])
		out.WriteString(edit.Replacement)
		cursor = edit.End
	}
	out.WriteString(text[cursor:])
	return out.String(), fixes, nil
}
