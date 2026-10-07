package lint

import (
	"fmt"
	"strings"

	"assx/internal/ass"
	"assx/internal/semantic"
)

func analyzeUndefinedStyleReferences(doc ass.Document) []Diagnostic {
	styles := semantic.DefinedStyleNames(doc.StyleFields)
	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		if name := strings.TrimSpace(dialogue.Style); name != "" && !semantic.ResolveDialogueStyleReference(name, styles) {
			diagnostics = append(diagnostics, undefinedStyleDiagnostic(
				dialogue.Line, dialogue.StyleColumn, "", "Style",
				fmt.Sprintf("Dialogue references undefined style %q.", name),
			))
		}
		for _, token := range dialogue.ParsedText().Tokens() {
			if token.Tag == nil || token.Tag.Name != "r" || len(token.Tag.Args) == 0 {
				continue
			}
			name := strings.TrimSpace(strings.Join(token.Tag.Args, ","))
			if name == "" || semantic.ResolveResetStyleReference(name, styles) {
				continue
			}
			diagnostics = append(diagnostics, undefinedStyleDiagnostic(
				dialogue.Line, token.Tag.Column, token.Tag.Name, "",
				fmt.Sprintf("Override tag references undefined style %q.", name),
			))
		}
	}
	return diagnostics
}

func undefinedStyleDiagnostic(line, column int, tag, field, detail string) Diagnostic {
	rule := Rules[IssueUndefinedStyle]
	return Diagnostic{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, Line: line, Column: column, Tag: tag, Field: field,
		Detail: detail, Sources: rule.Sources,
	}
}
