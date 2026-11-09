package lint

import (
	"assx/internal/ass/renderer"
)

func analyzeDocumentTest(doc renderer.Document) []Diagnostic {
	targets := DefaultAnalysisTargets()
	return proveSafeFixes(doc, analyzeDocumentForTargetsUnproved(doc, targets, nil), targets.Profiles())
}

func analyzeTargetsTest(doc renderer.Document, targets AnalysisTargets) []Diagnostic {
	return proveSafeFixes(doc, analyzeDocumentForTargetsUnproved(doc, targets, nil), targets.Profiles())
}

func analyzeRendererTest(doc renderer.Document, profile renderer.Profile) []Diagnostic {
	return scopeDiagnostics(analyzeDocumentForProfile(doc, profile, nil), profile)
}
