package lint

import "strings"

// Primary assertions remain independent of additional compatibility observations.
func primaryFindings(diagnostics []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, diagnostic := range diagnostics {
		if !strings.Contains(diagnostic.Renderer, " / ") {
			out = append(out, diagnostic)
		}
	}
	return out
}
