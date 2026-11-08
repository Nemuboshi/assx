package lint

// Primary assertions remain independent of additional compatibility observations.
func primaryFindings(diagnostics []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, diagnostic := range diagnostics {
		if !isBehaviorCondition(diagnostic.Condition) && diagnostic.Condition != IssueRendererUnresolved {
			out = append(out, diagnostic)
		}
	}
	return out
}
