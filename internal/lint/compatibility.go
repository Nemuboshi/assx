package lint

import (
	"fmt"
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
	return compatibilityDiagnostics(findings, func(span ass.ConcreteSpan) (int, int) { return dialogue.Line, span.Start + 1 })
}

// AnalyzeDocumentCompatibility uses the lossless document, including records
// the historical parser dropped. All positions refer to doc.Text byte offsets.
func AnalyzeDocumentCompatibility(doc ass.Document, profiles []renderer.Profile) []Diagnostic {
	tree := ass.ParseConcreteDocument(doc.Text)
	findings := semantic.CompareDocument(tree, profiles)
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
		span                       ass.ConcreteSpan
		dimension, targets, detail string
		status                     semantic.CompatibilityStatus
	}
	seen := make(map[key]bool)
	for _, finding := range findings {
		if finding.Status == semantic.CompatibilityEquivalent {
			continue
		}
		left, right := finding.Left.Profile, finding.Right.Profile
		scope := fmt.Sprintf("%s@%s (%+v) / %s@%s (%+v)", left.Kind(), left.Version(), left.Build(), right.Kind(), right.Version(), right.Build())
		k := key{finding.Source, finding.Dimension, scope, finding.Detail, finding.Status}
		if seen[k] {
			continue
		}
		seen[k] = true
		id := IssueRendererUnresolved
		if finding.Status == semantic.CompatibilityDivergent {
			id = IssueRendererDiff
		}
		rule := Rules[id]
		line, column := location(finding.Source)
		out = append(out, Diagnostic{ID: id, Severity: rule.Severity, Title: rule.Title, Description: "Compatibility observation for independently interpreted renderer profiles; separate from invocation validity and edit safety.", Fix: "Review both renderer interpretations and verify the intended rendering before changing the source.", Line: line, Column: column, Tag: finding.Left.Resolution.Head, Field: finding.Dimension, Detail: string(finding.Status) + ": " + finding.Detail, Renderer: scope, Sources: slices.Clone(finding.Citations)})
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
