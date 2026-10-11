package lint

import (
	"fmt"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

func analyzeUndefinedStyleReferencesForRenderer(doc ass.Document, profile renderer.Profile) []Diagnostic {
	return analyzeStyleReferences(doc, profile)
}

func analyzeStyleReferences(doc ass.Document, profile renderer.Profile) []Diagnostic {
	styles := semantic.DefinedStyleNames(doc.StyleFields)
	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		if name := strings.TrimSpace(dialogue.Style); name != "" && !semantic.ResolveDialogueStyleReference(name, styles) {
			diagnostics = append(diagnostics, undefinedStyleDiagnostic(
				dialogue.Line, dialogue.StyleColumn, "", "Style",
				fmt.Sprintf("Dialogue references undefined style %q.", name),
			))
		}
		checkReset := func(name string, column int) {
			name = strings.TrimSpace(name)
			if name == "" || semantic.ResolveResetStyleReference(name, styles) {
				return
			}
			diagnostics = append(diagnostics, undefinedStyleDiagnostic(
				dialogue.Line, column, "r", "",
				fmt.Sprintf("Override tag references undefined style %q.", name),
			))
		}
		profile.WalkDialogue(ass.ParseConcreteDialogue(dialogue.Text), func(result renderer.Result, nested bool) bool {
			if result.Status == renderer.Matched && result.Name == "r" && len(result.Args) > 0 {
				checkReset(result.Args[0].Raw, resolvedLintTag(result, nested).Column)
			}
			return true
		})
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
