package lint

import (
	"fmt"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

func analyzeUndefinedStyleReferences(doc ass.Document) []Diagnostic {
	return analyzeStyleReferences(doc, nil)
}

func analyzeUndefinedStyleReferencesForRenderer(doc ass.Document, profile renderer.Profile) []Diagnostic {
	return analyzeStyleReferences(doc, &profile)
}

func analyzeStyleReferences(doc ass.Document, profile *renderer.Profile) []Diagnostic {
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
		if profile == nil {
			dialogue.ParsedText().WalkTokens(func(token ass.TokenView) bool {
				if token.HasTag && token.Tag.Name == "r" && len(token.Tag.Args) > 0 {
					checkReset(strings.Join(token.Tag.Args, ","), token.Tag.Column)
				}
				return true
			})
		} else {
			profile.WalkDialogue(ass.ParseConcreteDialogue(dialogue.Text), func(result renderer.Result, _ bool) bool {
				if result.Status == renderer.Matched && result.Name == "r" && len(result.Args) > 0 {
					checkReset(result.Args[0].Raw, result.Source.Start+2)
				}
				return true
			})
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
