package lint

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// portabilityDiagnostics keeps ASS005 for Mod-only names in the strict scope.
// Name recognition does not authorize a Mod signature or an automatic edit.
func portabilityDiagnostics(dialogue ass.Dialogue, targets AnalysisTargets) []Diagnostic {
	for _, profile := range targets.Profiles() {
		if profile.Kind() == renderer.VSFilterMod {
			return nil
		}
	}
	mod, _ := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	analyzer := dialogueAnalyzer{line: dialogue.Line, textStart: dialogue.TextStart}
	mod.WalkDialogue(ass.ParseConcreteDialogue(dialogue.Text), func(result renderer.Result, nested bool) bool {
		if (result.Status == renderer.Matched || result.Status == renderer.Disabled) && result.HasPolicy && result.Policy.VSFilterModOnly {
			analyzer.add(IssueVSFilterModTag, resolvedLintTag(result, nested), "This VSFilterMod-only name is not portable to the selected renderer targets. Its dispatch or effects can differ; automatic edits require proof for every requested target.")
		}
		return true
	})
	return analyzer.diagnostics
}
