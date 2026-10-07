package lint

import (
	"fmt"
	"strings"

	"assx/internal/ass"
)

func analyzeUndefinedStyleReferences(doc ass.Document) []Diagnostic {
	styles := definedStyleNames(doc.StyleFields)
	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		if name := strings.TrimSpace(dialogue.Style); name != "" && !resolveDialogueStyleReference(name, styles) {
			diagnostics = append(diagnostics, undefinedStyleDiagnostic(
				dialogue.Line, dialogue.StyleColumn, "", "Style",
				fmt.Sprintf("Dialogue references undefined style %q.", name),
			))
		}
		for _, token := range Lex(dialogue.Text) {
			if token.Tag == nil || token.Tag.Name != "r" || len(token.Tag.Args) == 0 {
				continue
			}
			name := strings.TrimSpace(strings.Join(token.Tag.Args, ","))
			if name == "" || resolveResetStyleReference(name, styles) {
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

func definedStyleNames(fields []ass.StyleField) map[string]struct{} {
	styles := make(map[string]struct{})
	for _, field := range fields {
		if field.Name == "name" {
			styles[field.Value] = struct{}{}
		}
	}
	return styles
}

func resolveDialogueStyleReference(name string, styles map[string]struct{}) bool {
	name = strings.TrimLeft(strings.TrimSpace(name), "*")
	if strings.EqualFold(name, "Default") {
		name = "Default"
	}
	_, exists := styles[name]
	return exists
}

func resolveResetStyleReference(name string, styles map[string]struct{}) bool {
	_, exists := styles[strings.TrimSpace(name)]
	return exists
}

func undefinedStyleDiagnostic(line, column int, tag, field, detail string) Diagnostic {
	rule := Rules[IssueUndefinedStyle]
	return Diagnostic{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, Line: line, Column: column, Tag: tag, Field: field,
		Detail: detail, Sources: rule.Sources,
	}
}
