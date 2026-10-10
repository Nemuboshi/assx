package lint

import "assx/internal/edit"

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
	var edits []edit.TextEdit
	fixes := 0
	for _, diagnostic := range diagnostics {
		if diagnostic.FixSafety != SafeFix && (!includeUnsafe || diagnostic.FixSafety != UnsafeFix) {
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

	fixed, err := edit.Apply(text, edits)
	if err != nil {
		return "", 0, err
	}
	return fixed, fixes, nil
}
