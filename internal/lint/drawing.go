package lint

import "assx/internal/ass"

func analyzeDrawings(dialogue ass.Dialogue, tree ass.DialogueText) []Diagnostic {
	rule := Rules[IssueMalformedDrawing]
	var diagnostics []Diagnostic
	for _, drawing := range tree.Drawings() {
		for _, issue := range drawing.Issues {
			diagnostics = append(diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, Line: dialogue.Line, Column: issue.Start + 1,
				Detail: issue.Detail, Sources: rule.Sources,
			})
		}
	}
	return diagnostics
}
