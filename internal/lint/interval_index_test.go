package lint

import (
	"fmt"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/edit"
)

// scaledRedundantStyleDoc builds one dialogue per requested count. Every
// dialogue carries an override block that is wholly Style-redundant (ASS013)
// and whose two tags are also first-wins no-effect findings (ASS006) whose
// edits overlap the redundant block edit.
func scaledRedundantStyleDoc(dialogues int) ass.Document {
	var text strings.Builder
	text.WriteString("[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\n")
	text.WriteString("[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	text.WriteString("Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0,0,0,1\n")
	text.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	for i := range dialogues {
		fmt.Fprintf(&text, "Dialogue: 0,0:%02d:%02d.00,0:%02d:%02d.00,Default,,0,0,0,,{\\b0\\b0}plain line %d\n",
			i/60, i%60, i/60, (i%60)+1, i)
	}
	return ass.Parse(text.String())
}

// overlapWork runs the redundant-style suppression path for one document under
// the primary target and reports the interval comparisons it performed, the
// suppressed no-effect findings, and the redundant Style-override findings.
func overlapWork(t *testing.T, dialogues int) (comparisons int, noEffect, redundant int) {
	t.Helper()
	doc := scaledRedundantStyleDoc(dialogues)
	before := overlapComparisons.Load()
	findings := analyzeDocumentForProfile(doc, DefaultAnalysisTargets().Primary, nil)
	comparisons = int(overlapComparisons.Load() - before)
	for _, finding := range findings {
		switch finding.ID {
		case IssueNoEffect:
			noEffect++
		case IssueRedundantStyleOverrides:
			redundant++
		}
	}
	return comparisons, noEffect, redundant
}

// The suppression pass may not compare every candidate edit against every
// redundant edit: its work must grow with the document, not with the square of
// it. Interval comparisons are counted at two sizes.
func TestOverlapSuppressionDoesNotCompareEveryPair(t *testing.T) {
	small, smallNoEffect, smallRedundant := overlapWork(t, 250)
	large, largeNoEffect, largeRedundant := overlapWork(t, 1000)
	if smallNoEffect != 0 || largeNoEffect != 0 {
		t.Fatalf("suppression left no-effect findings: %d at 250, %d at 1000 dialogues", smallNoEffect, largeNoEffect)
	}
	if smallRedundant != 250 || largeRedundant != 1000 {
		t.Fatalf("redundant Style-override findings: %d at 250, %d at 1000 dialogues", smallRedundant, largeRedundant)
	}
	if large > small*8 {
		t.Fatalf("interval comparisons grew with the square of the document size: %d at 250 dialogues, %d at 1000", small, large)
	}
}

// intervalIndex must decide overlap exactly as the half-open predicate does.
// The corpus covers touching ranges, zero-width edits, containment, disjoint
// ranges, reversed ranges, and a deterministic random set.
func TestIntervalIndexMatchesHalfOpenPredicate(t *testing.T) {
	corpus := []edit.Interval{
		{Start: 0, End: 0}, {Start: 0, End: 1}, {Start: 1, End: 1}, {Start: 1, End: 3},
		{Start: 2, End: 5}, {Start: 3, End: 3}, {Start: 3, End: 5}, {Start: 4, End: 4},
		{Start: 5, End: 6}, {Start: 6, End: 6}, {Start: 1, End: 10}, {Start: 7, End: 2},
	}
	for _, redundant := range corpus {
		index := newIntervalIndex([]edit.Interval{redundant})
		for _, query := range corpus {
			want := intervalsOverlap(redundant, query)
			if got := index.overlapsAny(query); got != want {
				t.Fatalf("index.overlapsAny(%v) = %v, predicate = %v (redundant %v)", query, got, want, redundant)
			}
		}
	}

	const seed = 20261018
	state := uint64(seed)
	next := func(bound int) int {
		state = state*6364136223846793005 + 1442695040888963407
		return int(state>>33) % bound
	}
	const sets = 200
	for range sets {
		intervals := make([]edit.Interval, 1+next(12))
		for i := range intervals {
			start := next(24) - 4
			intervals[i] = edit.Interval{Start: start, End: start + next(6)}
		}
		index := newIntervalIndex(intervals)
		for range 32 {
			start := next(24) - 4
			probe := edit.Interval{Start: start, End: start + next(6)}
			want := false
			for _, interval := range intervals {
				if intervalsOverlap(interval, probe) {
					want = true
					break
				}
			}
			if got := index.overlapsAny(probe); got != want {
				t.Fatalf("index.overlapsAny(%v) = %v, predicate = %v, intervals %v", probe, got, want, intervals)
			}
		}
	}
}

func BenchmarkOverlapSuppressionScaling(b *testing.B) {
	for _, dialogues := range []int{250, 1000, 4000} {
		doc := scaledRedundantStyleDoc(dialogues)
		profile := DefaultAnalysisTargets().Primary
		b.Run(fmt.Sprintf("%d_dialogues", dialogues), func(b *testing.B) {
			b.ReportAllocs()
			before := overlapComparisons.Load()
			for i := 0; i < b.N; i++ {
				benchmarkDiagnostics = analyzeDocumentForProfile(doc, profile, nil)
			}
			b.ReportMetric(float64(overlapComparisons.Load()-before)/float64(b.N), "overlap/op")
		})
	}
}
