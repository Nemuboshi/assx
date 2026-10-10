package semantic

import (
	"testing"

	"assx/internal/ass"
)

// Explicit uncertainty reasons are independent of TagEvent.Known: special
// operations can be understood without producing a canonical assignment.
func TestSpecialTagUncertaintyEvents(t *testing.T) {
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{"fontsize": "20"}),
	}
	cases := []struct {
		name, text, tag string
		styles          bool
		inTransition    bool
		want            SemanticUncertainty
		barrier         bool
	}{
		{"generic assignment", "{\\fs20}A", "fs", false, false, UncertaintyNone, false},
		{"malformed position arity", "{\\pos(1)}A", "pos", false, false, UncertaintyMalformed, true},
		{"invalid alignment", "{\\an12}A", "an", false, false, UncertaintyMalformed, true},
		{"renderer-dependent alignment", "{\\a4}A", "a", false, false, UncertaintyRendererDependent, true},
		{"rectangular clip", "{\\clip(0,0,10,10)}A", "clip", false, false, UncertaintyNone, false},
		{"ambiguous rectangular clip", "{\\clip(1.5,0,10,10)}A", "clip", false, false, UncertaintyRendererDependent, true},
		{"vector clip not modeled", "{\\clip(m 0 0 l 10 10)}A", "clip", false, false, UncertaintyUnsupported, true},
		{"shared position first-wins", "{\\pos(1,2)\\move(3,4,5,6)}A", "move", false, false, UncertaintyNone, false},
		{"supported no-effect transform", "{\\bord2\\t(\\bord2)}A", "t", false, false, UncertaintyNone, false},
		{"time-dependent transform", "{\\bord2\\t(\\bord3)}A", "t", false, false, UncertaintyTimeDependent, false},
		{"malformed transform child", "{\\t(\\fsabc)}A", "fs", false, true, UncertaintyMalformed, true},
		{"unknown transform child", "{\\t(\\mystery)}A", "mystery", false, true, UncertaintyUnsupported, true},
		{"ordinary karaoke duration", "{\\k20}A", "k", false, false, UncertaintyNone, false},
		{"malformed karaoke duration", "{\\k20junk}A", "k", false, false, UncertaintyMalformed, true},
		{"absolute karaoke timing", "{\\kt20}A", "kt", false, false, UncertaintyRendererDependent, true},
		{"relative font size", "{\\fs+10}A", "fs", false, false, UncertaintyUnsupported, true},
		{"unknown tag", "{\\mystery}A", "mystery", false, false, UncertaintyUnsupported, true},
		{"renderer extension", "{\\1img}A", "1img", false, false, UncertaintyRendererDependent, true},
		{"valid Style reset", "{\\r}A", "r", true, false, UncertaintyNone, false},
		{"unresolved named Style reset", "{\\rMissing}A", "r", true, false, UncertaintyUnresolved, true},
		{"unresolved reset without Style table", "{\\rMissing}A", "r", false, false, UncertaintyUnresolved, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got TagEvent
			found := false
			options := EvaluationOptions{
				DialogueStyle: "Default",
				Observer: Observer{Tag: func(event TagEvent, _ StateView) {
					if event.Tag.Name == tc.tag && event.Tag.InTransition == tc.inTransition {
						got, found = event, true
					}
				}},
			}
			if tc.styles {
				options.Styles = styles
			}
			Evaluate(ass.ParseDialogueText(tc.text), options)
			if !found {
				t.Fatalf("missing event for tag %q in %q", tc.tag, tc.text)
			}
			if got.Uncertainty != tc.want || got.Barrier != tc.barrier {
				t.Fatalf("event uncertainty=%v barrier=%v; want uncertainty=%v barrier=%v: %#v",
					got.Uncertainty, got.Barrier, tc.want, tc.barrier, got)
			}
		})
	}
}

func TestSpecialTagUncertaintyRevokesPriorNoEffectProofs(t *testing.T) {
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{"fontsize": "20"}),
	}
	cases := []struct {
		name, text string
		styles     bool
	}{
		{"unknown named reset", "{\\fs20\\fs20}A{\\rMissing}B", true},
		{"absolute karaoke timing", "{\\fs20\\fs20}A{\\kt20}B", false},
		{"malformed karaoke duration", "{\\fs20\\fs20}A{\\k20junk}B", false},
		{"ambiguous rectangle", "{\\fs20\\fs20}A{\\clip(1.5,0,10,10)}B", false},
		{"unsupported vector clip", "{\\fs20\\fs20}A{\\clip(m 0 0 l 10 10)}B", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := EvaluationOptions{DialogueStyle: "Default"}
			if tc.styles {
				options.Styles = styles
			}
			effects := Evaluate(ass.ParseDialogueText(tc.text), options).NoEffects
			if len(effects) == 0 {
				t.Fatalf("no original no-effect candidate found for %q", tc.text)
			}
			for _, effect := range effects {
				if !effect.ProofRevoked {
					t.Fatalf("uncertain operation retained a SafeFix proof: %#v", effects)
				}
			}
		})
	}
}

func TestMalformedKaraokeNumericPrefixIsNotKnownTiming(t *testing.T) {
	Evaluate(ass.ParseDialogueText("{\\k20\\k20junk}A"), EvaluationOptions{
		Observer: Observer{Text: func(_ string, _ int, state StateView) {
			if cursor := state.Value("karaoke_cursor"); cursor.Known {
				t.Fatalf("numeric prefix was treated as known timing: %#v", cursor)
			}
			if source := state.Source("karaoke_cursor"); source != -1 {
				t.Fatalf("unknown karaoke timing retained stale source index %d", source)
			}
		}},
	})
}

// A malformed clip has no reliable rectangular/vector shape. Both effective
// slots and their provenance must be invalidated, including across Style reset.
func TestMalformedClipArityInvalidatesBothStateSlots(t *testing.T) {
	cases := []struct {
		name string
		text string
	}{
		{"clip after rectangular clip", "{\\clip(0,0,10,10)\\clip(1,2,3)}A"},
		{"iclip invalidates earlier clip", "{\\clip(0,0,10,10)\\iclip(1,2,3)}A"},
		{"clip invalidates earlier iclip", "{\\iclip(0,0,10,10)\\clip(1,2,3)}A"},
		{"unknown clip remains unknown after reset", "{\\clip(0,0,10,10)\\clip(1,2,3)\\r}A"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seenRect := false
			seenMalformed := false
			assertUnknown := func(label string, view StateView) {
				t.Helper()
				for _, slot := range []string{"clip_rect", "clip_vector"} {
					if value := view.Value(slot); value.Known {
						t.Errorf("%s: %s retained known state: %#v", label, slot, value)
					}
					if source := view.Source(slot); source != -1 {
						t.Errorf("%s: %s retained source tag %d", label, slot, source)
					}
				}
			}
			Evaluate(ass.ParseDialogueText(tc.text), EvaluationOptions{
				Observer: Observer{
					Tag: func(event TagEvent, state StateView) {
						if event.Index == 0 {
							if value := state.Value("clip_rect"); !value.Known {
								t.Errorf("initial rectangular clip was not known: %#v", value)
							}
							if source := state.Source("clip_rect"); source != 0 {
								t.Errorf("initial rectangular source = %d, want 0", source)
							}
							seenRect = true
						}
						if event.Index == 1 {
							if event.Uncertainty != UncertaintyMalformed || !event.Barrier {
								t.Errorf("malformed clip outcome = %#v", event)
							}
							assertUnknown("malformed clip tag", state)
							seenMalformed = true
						}
					},
					Text: func(_ string, _ int, state StateView) {
						assertUnknown("visible text", state)
					},
				},
			})
			if !seenRect || !seenMalformed {
				t.Fatalf("missing tag callbacks: rectangle=%v malformed=%v", seenRect, seenMalformed)
			}
		})
	}
}
