package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// The lint warning must never be more certain about the karaoke cursor than
// the single P05 state machine that all renderer-aware consumers share.
func TestReviewKaraokeCursorUsesP05State(t *testing.T) {
	lib, xy, mod, _, _ := scopedProfiles(t)
	for _, tc := range []struct {
		name        string
		profile     renderer.Profile
		text        string
		engineKnown bool
		cursorKnown bool
		want        int64
	}{
		{"verified libass", lib, `{\k200}A`, true, true, 2000},
		{"verified xy", xy, `{\kf100\ko100}A`, true, true, 2000},
		{"valid bare duration", lib, `{\k}A`, true, true, 1000},
		{"junk suffix", lib, `{\k200junk}A`, false, false, 0},
		{"incomplete number", lib, `{\k200.abc}A`, false, false, 0},
		{"known then malformed", xy, `{\k200\k200junk}A`, false, false, 0},
		{"unresolved after syllable", lib, `{\k200\t(0,100}A`, false, false, 0},
		{"inferred VSFilterMod", mod, `{\k200}A`, true, false, 0},
		{"inferred bare VSFilterMod", mod, `{\k}A`, true, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := ass.ParseConcreteDialogue(tc.text)
			var last semantic.StateValue
			semantic.EvaluateResolved(tree, tc.profile, semantic.EvaluationOptions{
				Observer: semantic.Observer{
					Tag: func(_ semantic.TagEvent, state semantic.StateView) {
						last = state.Value("karaoke_cursor")
					},
					Text: func(_ string, _ int, state semantic.StateView) {
						last = state.Value("karaoke_cursor")
					},
				},
			})
			if last.Known != tc.engineKnown {
				t.Fatalf("unexpected P05 state: %#v, want known=%v", last, tc.engineKnown)
			}
			got, ok := karaokeCursorResolved(tree, tc.profile)
			if ok != tc.cursorKnown || (ok && got != tc.want) {
				t.Fatalf("ASS030 cursor=%d known=%v, want %d known=%v (P05=%#v)",
					got, ok, tc.want, tc.cursorKnown, last)
			}
		})
	}
}

func TestReviewRendererScopedKaraokeDocumentWarnings(t *testing.T) {
	lib, xy, mod, _, _ := scopedProfiles(t)
	for _, tc := range []struct {
		name       string
		profile    renderer.Profile
		text       string
		want       bool
		unresolved bool
	}{
		{"libass valid overrun", lib, `{\k200}A`, true, false},
		{"xy valid overrun", xy, `{\kf100\ko100}A`, true, false},
		{"libass numeric junk", lib, `{\k200junk}A`, false, false},
		{"xy fractional junk", xy, `{\k200.abc}A`, false, false},
		{"mod inferred signature", mod, `{\k200}A`, false, true},
		{"mod inferred mixed", mod, `{\kf100\k100}A`, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := eventDocument(
				"Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text",
				standardEventDialogue("0:00:00.00", "0:00:00.50", tc.text),
			)
			full := AnalyzeDocumentForRenderer(doc, tc.profile)
			eventOnly := AnalyzeEventFieldsForRenderer(doc, tc.profile)
			for _, ds := range [][]Diagnostic{full, eventOnly} {
				count := 0
				for _, d := range ds {
					if d.ID != IssueKaraoke {
						continue
					}
					count++
					if d.Renderer != tc.profile.Kind().String() || d.FixSafety != "" ||
						len(d.Edits) != 0 || !strings.Contains(d.Detail, "2000 ms") {
						t.Fatalf("incorrect scoped ASS030: %#v", d)
					}
				}
				want := 0
				if tc.want {
					want = 1
				}
				if count != want {
					t.Fatalf("wanted %d ASS030 findings, found %d: %#v", want, count, ds)
				}
			}
			if hasIssue(full, IssueRendererUnresolved) != tc.unresolved {
				t.Fatalf("ASS031 expectation=%v, findings=%#v", tc.unresolved, full)
			}
		})
	}
}

func TestReviewDefaultKaraokeContractUnchanged(t *testing.T) {
	doc := eventDocument(
		"Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text",
		standardEventDialogue("0:00:00.00", "0:00:00.50", `{\k200}A`),
	)
	if !hasIssue(AnalyzeEventFields(doc), IssueKaraoke) ||
		!hasIssue(AnalyzeDocument(doc), IssueKaraoke) {
		t.Fatal("frozen default karaoke warning changed")
	}
}
