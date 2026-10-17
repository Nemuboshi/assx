package lint

import (
	"context"

	"assx/internal/ass"
	editpkg "assx/internal/edit"
	"sort"
)

// AnalyzeDocumentForTargets validates only the primary renderer and reports
// compatibility observations for the full selected profile set separately.
func AnalyzeDocumentForTargets(doc ass.Document, targets AnalysisTargets) []Diagnostic {
	return proveSafeFixes(doc, analyzeDocumentForTargetsUnproved(doc, targets, nil), targets.Profiles())
}

func analyzeDocumentForTargetsUnproved(doc ass.Document, targets AnalysisTargets, policy RulePolicy) []Diagnostic {
	findings, _ := analyzeDocumentForTargetsUnprovedContext(context.Background(), doc, targets, policy)
	return findings
}

func analyzeDocumentForTargetsUnprovedContext(ctx context.Context, doc ass.Document, targets AnalysisTargets, policy RulePolicy) ([]Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	findings, err := analyzeDocumentForProfileContext(ctx, doc, targets.Primary, policy)
	if err != nil {
		return nil, err
	}
	for _, dialogue := range doc.Dialogues {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		findings = append(findings, portabilityDiagnostics(dialogue, targets)...)
	}
	findings = append(findings, AnalyzeDocumentCompatibility(doc, targets.Profiles())...)
	for i := range findings {
		if findings[i].Renderer == "" {
			findings[i].Renderer = ProfileScope(targets.Primary)
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line == findings[j].Line {
			return findings[i].Column < findings[j].Column
		}
		return findings[i].Line < findings[j].Line
	})
	return policy.apply(findings), nil
}

func AnalyzeDocument(doc ass.Document) []Diagnostic {
	return AnalyzeDocumentForTargets(doc, DefaultAnalysisTargets())
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
					if (editpkg.Interval{Start: edit.Start, End: edit.End}).Overlaps(editpkg.Interval{Start: fontEdit.Start, End: fontEdit.End}) {
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
