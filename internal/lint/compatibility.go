package lint

import (
	"slices"
	"sort"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// AnalyzeCompatibility reports differential observations separately from
// validity analysis. Nil targets selects libass + xy-VSFilter. The typed
// semantic comparison also exposes equivalent observations for API consumers.
// Compatibility findings never authorize edits or change default Analyze.
func AnalyzeCompatibility(dialogue ass.Dialogue, profiles []renderer.Profile) []Diagnostic {
	findings := semantic.CompareDialogue(ass.ParseConcreteDialogue(dialogue.Text), profiles, semantic.EvaluationOptions{})
	diagnostics := compatibilityDiagnostics(findings, func(span ass.ConcreteSpan) (int, int) { return dialogue.Line, span.Start + 1 })
	for i := range diagnostics {
		span := diagnostics[i].SourceSpan.AtDocumentOffset(dialogue.TextStart)
		diagnostics[i].SourceSpan = &span
	}
	return diagnostics
}

// AnalyzeDocumentCompatibility uses the lossless document, including records
// the document view cannot represent. All positions refer to doc.Text byte offsets.
func AnalyzeDocumentCompatibility(doc ass.Document, profiles []renderer.Profile) []Diagnostic {
	return analyzeDocumentCompatibility(doc, nil, profiles, semantic.ComparisonMode{})
}

// analyzeDocumentCompatibility converts findings with a request-supplied
// concrete document and comparison mode. A nil tree parses doc.Text.
// A single profile has no pair to compare, so it emits no finding.
func analyzeDocumentCompatibility(doc ass.Document, tree *ass.ConcreteDocument, profiles []renderer.Profile, mode semantic.ComparisonMode) []Diagnostic {
	if len(profiles) == 1 {
		return nil
	}
	if tree == nil {
		parsed := ass.ParseConcreteDocument(doc.Text)
		tree = &parsed
	}
	findings := semantic.CompareDocumentInMode(*tree, profiles, mode)
	return compatibilityDiagnostics(findings, func(span ass.ConcreteSpan) (int, int) {
		index := sort.Search(len(tree.Lines), func(i int) bool { return tree.Lines[i].Span.End > span.Start })
		if index == len(tree.Lines) {
			return 1, span.Start + 1
		}
		line := tree.Lines[index]
		return line.Number, span.Start - line.ContentSpan.Start + 1
	})
}

func compatibilityDiagnostics(findings []semantic.CompatibilityFinding, location func(ass.ConcreteSpan) (int, int)) []Diagnostic {
	var out []Diagnostic
	type key struct {
		span              ass.ConcreteSpan
		condition, detail string
		status            semantic.CompatibilityStatus
	}
	seen := make(map[key]bool)
	for _, finding := range findings {
		if finding.Status == semantic.CompatibilityEquivalent {
			continue
		}
		condition := behaviorCondition(finding.Dimension)
		if finding.Status != semantic.CompatibilityDivergent {
			condition = IssueRendererUnresolved
		}
		k := key{finding.Source, condition, finding.Detail, finding.Status}
		if seen[k] {
			continue
		}
		seen[k] = true
		rule := Rules[condition]
		line, column := location(finding.Source)
		out = append(out, Diagnostic{
			Condition: condition, Scenario: finding.Dimension,
			Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, Line: line, Column: column, Tag: finding.Left.Resolution.Name,
			Field: finding.Dimension, Detail: string(finding.Status) + ": " + finding.Detail,
			Renderers:  compatibilityObservations(finding),
			SourceSpan: &finding.Source, Sources: slices.Clone(finding.Citations), spanJSON: true,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Line == out[j].Line {
			return out[i].Column < out[j].Column
		}
		return out[i].Line < out[j].Line
	})
	// Normalize absent citations to an empty array for explicit evidence absence.
	for i := range out {
		if out[i].Sources == nil {
			out[i].Sources = []string{}
		}
		out[i].Tag = strings.TrimSpace(out[i].Tag)
	}
	return out
}
