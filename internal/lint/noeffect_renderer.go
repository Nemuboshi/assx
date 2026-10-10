package lint

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// AnalyzeNoEffectsForRenderer exposes ASS006 observations for one pinned
// renderer without coupling them to legacy name/arity validation. P06 will
// integrate renderer-specific diagnostics. P08 will establish which observations
// justify multi-target edits; this function never attaches an automatic fix.
func AnalyzeNoEffectsForRenderer(dialogue ass.Dialogue, profile renderer.Profile) []Diagnostic {
	evaluation := semantic.EvaluateResolved(ass.ParseConcreteDialogue(dialogue.Text), profile,
		semantic.EvaluationOptions{DialogueStyle: dialogue.Style})
	rule := Rules[IssueNoEffect]
	findings := make([]Diagnostic, 0, len(evaluation.NoEffects))
	for _, effect := range evaluation.NoEffects {
		findings = append(findings, Diagnostic{
			ID: rule.ID, Severity: rule.Severity,
			Title: rule.Title, Description: rule.Description, Fix: rule.Fix,
			Line: dialogue.Line, Column: effect.Tag.Column,
			Tag: effect.Tag.Name, Detail: noEffectDetail(effect),
			Sources: rule.Sources,
		})
	}
	return findings
}
