package lint

import (
	"context"

	"assx/internal/ass"
	editpkg "assx/internal/edit"
	"sort"
	"sync/atomic"
)

// overlapComparisons counts the interval decisions the redundant-Style
// suppression pass performs: one per candidate/redundant pair while the pass
// scanned every pair, one per candidate query under the interval index. Tests
// watch it to prove the pass decides overlap without comparing every
// candidate edit against every redundant edit.
var overlapComparisons atomic.Int64

// intervalsOverlap is the half-open overlap predicate the suppression pass
// answers with, kept as the reference the interval index is tested against.
func intervalsOverlap(a, b editpkg.Interval) bool {
	overlapComparisons.Add(1)
	return a.Overlaps(b)
}

// intervalIndex answers "does any indexed interval overlap this one?" from a
// start-sorted list and the running maximum end. A candidate satisfies the
// half-open predicate exactly when its start precedes the query's end and its
// end follows the query's start, so one binary search plus one prefix maximum
// decides the whole set.
type intervalIndex struct {
	starts []int
	maxEnd []int
}

func newIntervalIndex(intervals []editpkg.Interval) *intervalIndex {
	ordered := append([]editpkg.Interval(nil), intervals...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Start < ordered[j].Start })
	index := &intervalIndex{starts: make([]int, len(ordered)), maxEnd: make([]int, len(ordered))}
	for i, interval := range ordered {
		index.starts[i] = interval.Start
		index.maxEnd[i] = interval.End
		if i > 0 && index.maxEnd[i-1] > index.maxEnd[i] {
			index.maxEnd[i] = index.maxEnd[i-1]
		}
	}
	return index
}

func (index *intervalIndex) overlapsAny(query editpkg.Interval) bool {
	overlapComparisons.Add(1)
	candidates := sort.Search(len(index.starts), func(i int) bool { return index.starts[i] >= query.End })
	return candidates != 0 && index.maxEnd[candidates-1] > query.Start
}

// AnalyzeDocumentForTargets validates only the primary renderer and reports
// compatibility observations for the full selected profile set separately.
func AnalyzeDocumentForTargets(doc ass.Document, targets AnalysisTargets) []Diagnostic {
	return proveSafeFixes(doc, analyzeDocumentForTargetsUnproved(doc, targets, nil), targets.Profiles())
}

func analyzeDocumentForTargetsUnproved(doc ass.Document, targets AnalysisTargets, policy RulePolicy) []Diagnostic {
	findings, _ := analyzeDocumentForTargetsUnprovedContext(context.Background(), doc, targets, policy, nil)
	return findings
}

// analyzeDocumentForTargetsUnprovedContext runs discovery with optional
// request preparation. A shared analysis supplies the concrete document, the
// projected views, and the dialogue operations, so each profile resolves a
// given expression once per request instead of once per consumer.
func analyzeDocumentForTargetsUnprovedContext(ctx context.Context, doc ass.Document, targets AnalysisTargets, policy RulePolicy, shared *DocumentAnalysis) ([]Diagnostic, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	findings, err := analyzeDocumentForProfileContext(ctx, doc, targets.Primary, policy, shared)
	if err != nil {
		return nil, err
	}
	for _, dialogue := range doc.Dialogues {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		findings = append(findings, portabilityDiagnostics(dialogue, targets)...)
	}
	tree, _ := shared.concreteDocument(doc)
	findings = append(findings, analyzeDocumentCompatibility(doc, tree, targets.Profiles(), shared.compatibilityMode())...)
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

// redundantIntervals flattens the redundant findings' edits into the interval
// set the suppression index answers from.
func redundantIntervals(redundant []Diagnostic) []editpkg.Interval {
	var intervals []editpkg.Interval
	for _, diagnostic := range redundant {
		for _, sourceEdit := range diagnostic.Edits {
			intervals = append(intervals, editpkg.Interval{Start: sourceEdit.Start, End: sourceEdit.End})
		}
	}
	return intervals
}

// suppressOverlappingNoEffect drops a no-effect edit when it overlaps any
// redundant Style-override edit. The index is built on first use, so a request
// with no no-effect candidate keeps its previous cost.
func suppressOverlappingNoEffect(diagnostics, redundant []Diagnostic) []Diagnostic {
	filtered := make([]Diagnostic, 0, len(diagnostics))
	var index *intervalIndex
	for _, diagnostic := range diagnostics {
		if diagnostic.ID != IssueNoEffect || len(diagnostic.Edits) == 0 {
			filtered = append(filtered, diagnostic)
			continue
		}
		kept := make([]TextEdit, 0, len(diagnostic.Edits))
		for _, candidate := range diagnostic.Edits {
			if index == nil {
				index = newIntervalIndex(redundantIntervals(redundant))
			}
			if !index.overlapsAny(editpkg.Interval{Start: candidate.Start, End: candidate.End}) {
				kept = append(kept, candidate)
			}
		}
		if len(kept) != 0 {
			diagnostic.Edits = kept
			filtered = append(filtered, diagnostic)
		}
	}
	return filtered
}
