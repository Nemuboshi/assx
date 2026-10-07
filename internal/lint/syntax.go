package lint

import (
	"fmt"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

func analyzeOverrideSyntax(dialogue ass.Dialogue, tree ass.DialogueText) []Diagnostic {
	var diagnostics []Diagnostic
	for _, node := range tree.Nodes {
		if node.Kind != ass.OverrideNode || node.Block == nil {
			continue
		}
		block := node.Block
		if block.IsEmpty() {
			rule := Rules[IssueEmptyOverrideBlock]
			diagnostics = append(diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, FixSafety: rule.FixSafety, Line: dialogue.Line, Column: block.Start + 1,
				Edits:   []TextEdit{{Start: dialogue.TextStart + block.Start, End: dialogue.TextStart + block.End}},
				Sources: rule.Sources,
			})
			continue
		}

		for _, item := range block.Items {
			if item.Tag == nil {
				if junk := meaningfulRawJunk(item.Raw); junk != "" {
					diagnostics = append(diagnostics, overrideJunkDiagnostic(dialogue, item.Start+1, junk))
				}
				continue
			}
			if tail := ignoredTagTail(*item.Tag); tail != "" {
				diagnostics = append(diagnostics, overrideJunkDiagnostic(dialogue, item.Tag.Column, tail))
			}
		}
	}
	return diagnostics
}

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

func ignoredTagTail(tag ass.Tag) string {
	if tag.Paren || len(tag.Args) != 1 {
		return ""
	}
	tagSpec, ok := spec.TagSpecs[tag.Name]
	if !ok {
		return ""
	}

	arg := tag.Args[0]
	var match []int
	switch tagSpec.Value {
	case spec.IntegerValue, spec.BoldValue:
		match = integerPrefix.FindStringIndex(arg)
	case spec.NumberValue:
		match = numberPrefix.FindStringIndex(arg)
	case spec.HexValue:
		value := strings.TrimLeft(arg, " \t")
		prefix := 0
		if len(value) >= 2 && strings.EqualFold(value[:2], "&H") {
			prefix = 2
			value = value[2:]
		}
		hex := hexPrefix.FindStringIndex(value)
		if hex == nil {
			return ""
		}
		startSpaces := len(arg) - len(strings.TrimLeft(arg, " \t"))
		match = []int{0, startSpaces + prefix + hex[1]}
	default:
		return ""
	}
	if match == nil {
		return ""
	}
	tail := strings.TrimSpace(arg[match[1]:])
	if tail == "" {
		return ""
	}
	if tagSpec.Value == spec.HexValue {
		tail = strings.TrimPrefix(tail, "&")
		tail = strings.TrimSpace(tail)
	}
	return tail
}
