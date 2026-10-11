package lint

import (
	"fmt"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

func overrideJunkDiagnostic(dialogue ass.Dialogue, column int, junk string) Diagnostic {
	rule := Rules[IssueOverrideJunk]
	return Diagnostic{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, Line: dialogue.Line, Column: column,
		Detail: fmt.Sprintf("Ignored data: %q.", junk), Sources: rule.Sources,
	}
}

func meaningfulRawJunk(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.Trim(trimmed, "\\") == "" {
		return ""
	}
	return trimmed
}

// ignoredTagTailIR never resolves names. Renderer-scoped consumers supply the
// precise policy already chosen by their Profile.
func ignoredTagTailIR(tag ass.Tag, ir ass.TagIR) string {
	if tag.Paren || len(tag.Args) != 1 || !ir.Known {
		return ""
	}
	switch ir.Spec.Value {
	case spec.IntegerValue, spec.BoldValue, spec.NumberValue, spec.HexValue:
	default:
		return ""
	}
	arg := tag.Args[0]
	decoded := ir.Argument(0)
	if decoded.Consumed == 0 || decoded.Consumed > len(arg) {
		return ""
	}
	tail := strings.TrimSpace(arg[decoded.Consumed:])
	if ir.Spec.Value == spec.HexValue {
		tail = strings.TrimSpace(strings.TrimPrefix(tail, "&"))
	}
	return tail
}
