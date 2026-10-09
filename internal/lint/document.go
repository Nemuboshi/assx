package lint

import (
	"sort"

	"assx/internal/ass"
)

func AnalyzeDocument(doc ass.Document) []Diagnostic {
	diagnostics := AnalyzeHeaders(doc)
	diagnostics = append(diagnostics, AnalyzeKeywords(doc)...)
	diagnostics = append(diagnostics, AnalyzeStyles(doc)...)
	diagnostics = append(diagnostics, AnalyzeEventFields(doc)...)
	diagnostics = append(diagnostics, analyzeUndefinedStyleReferences(doc)...)
	for _, dialogue := range doc.Dialogues {
		diagnostics = append(diagnostics, Analyze(dialogue)...)
	}

	styleDiagnostics := AnalyzeRedundantStyleOverrides(doc)
	if len(styleDiagnostics) != 0 {
		diagnostics = suppressOverlappingNoEffect(diagnostics, styleDiagnostics)
		diagnostics = append(diagnostics, styleDiagnostics...)
	}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		if diagnostics[i].Line == diagnostics[j].Line {
			return diagnostics[i].Column < diagnostics[j].Column
		}
		return diagnostics[i].Line < diagnostics[j].Line
	})
	return diagnostics
}

func suppressOverlappingNoEffect(diagnostics, redundant []Diagnostic) []Diagnostic {
	filtered := make([]Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.ID != IssueNoEffect || len(diagnostic.Edits) == 0 {
			filtered = append(filtered, diagnostic)
			continue
		}
		kept := make([]TextEdit, 0, len(diagnostic.Edits))
		for _, edit := range diagnostic.Edits {
			overlaps := false
			for _, fontDiagnostic := range redundant {
				for _, fontEdit := range fontDiagnostic.Edits {
					if edit.Start < fontEdit.End && fontEdit.Start < edit.End {
						overlaps = true
						break
					}
				}
				if overlaps {
					break
				}
			}
			if !overlaps {
				kept = append(kept, edit)
			}
		}
		if len(kept) != 0 {
			diagnostic.Edits = kept
			filtered = append(filtered, diagnostic)
		}
	}
	return filtered
}
