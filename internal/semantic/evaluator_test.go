package semantic

import (
	"testing"

	"assx/internal/ass"
)

// These tests exercise the public event stream shared by ASS006, ASS013 and
// font analysis, rather than testing three independent tag interpreters.
func TestEvaluatorTextBoundariesResetsAndSources(t *testing.T) {
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{
			"fontname": "Arial", "fontsize": "20", "bold": "0", "outline": "2",
		}),
		"Other": CanonicalStyleState(map[string]string{
			"fontname": "Courier New", "fontsize": "30", "bold": "-1", "outline": "4",
		}),
	}
	tree := ass.ParseDialogueText("{\\fs24\\bord3}A{\\rOther}B{\\fs0}C{\\r}D")
	type snapshot struct {
		text, family, fontSize string
		known, weightKnown     bool
		fontSource             int
	}
	var got []snapshot
	evaluation := Evaluate(tree, EvaluationOptions{
		Styles: styles, DialogueStyle: "Default",
		Observer: Observer{Text: func(text string, _ int, view StateView) {
			font := view.Value("fontname")
			size := view.Value("fontsize")
			bold := view.Value("bold")
			got = append(got, snapshot{
				text: text, family: font.Value, fontSize: size.Value,
				known: size.Known, weightKnown: bold.Known,
				fontSource: view.Source("fontsize"),
			})
		}},
	})
	if len(got) != 4 {
		t.Fatalf("text snapshots = %#v", got)
	}
	if got[0].text != "A" || got[0].fontSize == "" || !got[0].known || got[0].fontSource < 0 {
		t.Fatalf("explicit override was not tracked: %#v", got[0])
	}
	if got[1].text != "B" || got[1].family != "Courier New" || got[1].fontSource != -1 || !got[1].weightKnown {
		t.Fatalf("named reset did not install the named Style: %#v", got[1])
	}
	if got[2].text != "C" || got[2].known {
		t.Fatalf("renderer-dependent reset was incorrectly proven: %#v", got[2])
	}
	if got[3].text != "D" || got[3].family != "Arial" || !got[3].known || got[3].fontSource != -1 {
		t.Fatalf("plain reset did not restore the Dialogue Style: %#v", got[3])
	}
	if len(evaluation.NoEffects) > 0 {
		t.Fatalf("reset/used tags were incorrectly classified: %#v", evaluation.NoEffects)
	}
}

func TestEvaluatorPrecedenceAndProvenance(t *testing.T) {
	tree := ass.ParseDialogueText("{\\an7\\an8\\xbord2\\ybord2\\bord2}A{\\r\\an9}B")
	var alignment, border StateValue
	var alignmentOwner int
	evaluation := Evaluate(tree, EvaluationOptions{
		Observer: Observer{Text: func(text string, _ int, view StateView) {
			if text == "A" {
				border = view.Value("border_x")
			}
			if text == "B" {
				alignment = view.Value("alignment")
				alignmentOwner = view.Source("alignment")
			}
		}},
	})
	if !alignment.Known || alignment.Value != "7" || !border.Known || alignmentOwner != 0 {
		t.Fatalf("reset changed first-wins provenance or lost border: alignment=%#v border=%#v owner=%d", alignment, border, alignmentOwner)
	}
	var ignored, same bool
	for _, effect := range evaluation.NoEffects {
		if effect.Tag.Name == "an" && effect.Reason == FirstWinsIgnored {
			ignored = true
		}
		if effect.Tag.Name == "bord" && effect.Reason == SameValue {
			same = true
		}
	}
	if !ignored || !same {
		t.Fatalf("first-wins or multi-slot same-value proof missing: %#v", evaluation.NoEffects)
	}
}

func TestEvaluatorAccumulatesAndRetainsKaraokeAcrossStyleResets(t *testing.T) {
	tree := ass.ParseDialogueText("{\\k20}A{\\kf30}B{\\r}C")
	var values []string
	Evaluate(tree, EvaluationOptions{
		Observer: Observer{Text: func(_ string, _ int, state StateView) {
			cursor := state.Value("karaoke_cursor")
			if !cursor.Known {
				t.Fatalf("lost known karaoke cursor: %#v", cursor)
			}
			values = append(values, cursor.Value)
		}},
		SkipNoEffectProofs: true,
	})
	if len(values) != 3 || values[0] != "200" || values[1] != "500" || values[2] != "500" {
		t.Fatalf("karaoke accumulation = %q", values)
	}
	for _, text := range []string{
		"{\\k20\\kt50\\k20}A",
		"{\\k20\\k999999999999999999999}A",
		"{\\k20\\t(\\k20)}A",
	} {
		Evaluate(ass.ParseDialogueText(text), EvaluationOptions{
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				if state.Value("karaoke_cursor").Known {
					t.Errorf("%q incorrectly produced known karaoke timing", text)
				}
			}},
		})
	}
}

func TestEvaluatorPropagatesUnknownAndRecoversAfterReset(t *testing.T) {
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{
			"fontname": "Arial", "fontsize": "20", "bold": "0",
		}),
	}
	for _, text := range []string{
		"{\\fnCourier New\\blur101}A{\\r}B",
		"{\\fnCourier New\\mystery}A{\\r}B",
		"{\\fnCourier New\\t(\\fs30)}A{\\r}B",
	} {
		var seen int
		Evaluate(ass.ParseDialogueText(text), EvaluationOptions{
			Styles: styles, DialogueStyle: "Default", SkipNoEffectProofs: true,
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				font := state.Value("fontname")
				if seen == 0 && font.Known && text != "{\\fnCourier New\\blur101}A{\\r}B" {
					t.Errorf("unsafe known state after unknown operation %q: %#v", text, font)
				}
				if seen == 1 && (!font.Known || font.Value != "Arial") {
					t.Errorf("reset failed to recover Style state %q: %#v", text, font)
				}
				seen++
			}},
		})
		if seen != 2 {
			t.Fatalf("text boundaries = %d for %q", seen, text)
		}
	}
}

func TestEvaluatorSkipsProvenanceWorkForFontOnlyReads(t *testing.T) {
	tree := ass.ParseDialogueText("{\\fnMissing\\fnArial\\p1}m 0 0{\\p0}A")
	var visibleFont string
	result := Evaluate(tree, EvaluationOptions{
		SkipNoEffectProofs: true,
		Observer: Observer{Text: func(text string, _ int, state StateView) {
			if text == "A" {
				visibleFont = state.Value("fontname").Value
			}
		}},
	})
	if len(result.NoEffects) != 0 || visibleFont != "Arial" {
		t.Fatalf("font-only mode returned effects or lost active font: result=%#v font=%q", result, visibleFont)
	}
}

func TestEvaluatorRejectsUncertainFirstWinsAndUnknownOwnerProofs(t *testing.T) {
	cases := []struct {
		name, text string
	}{
		{"integer overflow", "{\\an9999999999999999999999\\an7}A"},
		{"out of range", "{\\an12\\an7}A"},
		{"ambiguous legacy alignment", "{\\a4\\an7}A"},
		{"ambiguous rectangle clip", "{\\clip(0.5,0,10,10)\\clip(0,0,20,20)}A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var source int
			effects := Evaluate(ass.ParseDialogueText(tc.text), EvaluationOptions{
				Observer: Observer{Text: func(_ string, _ int, view StateView) {
					source = view.Source("alignment")
				}},
			})
			for _, effect := range effects.NoEffects {
				if effect.Reason == FirstWinsIgnored {
					t.Errorf("unsafe first-wins fix under uncertain owner: %#v", effect)
				}
			}
			if tc.name != "ambiguous rectangle clip" && source != -1 {
				t.Errorf("uncertain tag unexpectedly became a proven source: %d", source)
			}
		})
	}
}

func TestEvaluatorUnknownAssignmentIsNotProvenOverwritten(t *testing.T) {
	for _, tc := range []string{
		"{\\fs1e309\\fs20}A",
		"{\\blur101\\blur2}A",
		"{\\fs20junk\\fs24}A",
		"{\\fs　20\\fs24}A",
	} {
		effects := EvaluateDialogue(ass.ParseDialogueText(tc))
		for _, effect := range effects {
			if effect.Reason == OverwrittenBeforeUse {
				t.Errorf("%q generated an overwrite proof from an unknown tag: %#v", tc, effect)
			}
		}
	}
}
