package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// TestInteractionMatrixEffectiveState pins the effective state at every
// visible text run for mixed special tags. It covers the interaction groups
// from the SafeFix test issue: nested transforms, named and bare resets,
// clip/iclip, pos/move, drawing mode, colour/alpha, karaoke, and unknown
// propagation across text boundaries.
func TestInteractionMatrixEffectiveState(t *testing.T) {
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{
			"fontname": "Arial", "fontsize": "20", "bold": "0", "italic": "0",
		}),
		"Other": CanonicalStyleState(map[string]string{
			"fontname": "Courier New", "fontsize": "24", "bold": "1", "italic": "0",
		}),
	}
	cases := []struct {
		name  string
		mod   bool
		text  string
		texts []interactionText
	}{
		{
			name: "transform invalidates state then bare reset",
			text: `{\fs24\bord2\t(\bord4)}A{\r}B`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"fontsize": {Known: false}}},
				{text: "B", values: map[string]ProofValue{"fontsize": {Value: "20:20", Known: true}}, sources: map[string]int{"fontsize": -1}},
			},
		},
		{
			name: "named reset then bare reset",
			text: `{\rOther}A{\r}B`,
			texts: []interactionText{
				{
					text: "A", activeStyle: "Other",
					values:  map[string]ProofValue{"fontsize": {Value: "24:24", Known: true}, "bold": {Value: "1", Known: true}},
					sources: map[string]int{"fontsize": -1, "bold": -1},
				},
				{
					text: "B", activeStyle: "Default",
					values:  map[string]ProofValue{"fontsize": {Value: "20:20", Known: true}, "bold": {Value: "0", Known: true}},
					sources: map[string]int{"fontsize": -1, "bold": -1},
				},
			},
		},
		{
			name: "clip then inverse clip",
			text: `{\clip(0,0,10,10)}A{\iclip(1,2,30,40)}B`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"clip_rect": {Value: "clip:0:0,0:0,10:10,10:10", Known: true}}, sources: map[string]int{"clip_rect": 0}},
				{text: "B", values: map[string]ProofValue{"clip_rect": {Value: "iclip:1:1,2:2,30:30,40:40", Known: true}}, sources: map[string]int{"clip_rect": 1}},
			},
		},
		{
			name: "position first wins over move",
			text: `{\pos(1,2)\move(0,0,100,100)}A`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"position": {Value: "1:1,2:2", Known: true}}, sources: map[string]int{"position": 0}},
			},
		},
		{
			name: "drawing mode enters and leaves",
			text: `{\p1}m 0 0 l 10 10{\p0}A`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"drawing_scale": {Value: "0", Known: true}}, sources: map[string]int{"drawing_scale": 1}},
			},
		},
		{
			name: "colour alpha then reset",
			text: "{\\1c&HFF0000&\\1a&H80&}A{\\r}B",
			texts: []interactionText{
				{
					text: "A",
					values: map[string]ProofValue{
						"c1": {Value: "ff0000", Known: true},
						"a1": {Value: "80", Known: true},
					},
					sources: map[string]int{"c1": 0, "a1": 1},
				},
				{text: "B", activeStyle: "Default", values: map[string]ProofValue{"c1": {Known: false}, "a1": {Known: false}}},
			},
		},
		{
			name: "karaoke accumulates across resets",
			text: `{\k20}A{\kf30}B{\r}C`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"karaoke_cursor": {Value: "200", Known: true}}},
				{text: "B", values: map[string]ProofValue{"karaoke_cursor": {Value: "500", Known: true}}},
				{text: "C", values: map[string]ProofValue{"karaoke_cursor": {Value: "500", Known: true}}},
			},
		},
		{
			name: "unknown tag invalidates state until reset",
			text: `{\fs24\mystery}A{\r}B`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"fontsize": {Known: false}}},
				{text: "B", values: map[string]ProofValue{"fontsize": {Value: "20:20", Known: true}}, sources: map[string]int{"fontsize": -1}},
			},
		},
		{
			name: "malformed transform child invalidates state until reset",
			text: `{\fs24\t(\fsabc)}A{\r}B`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"fontsize": {Known: false}}},
				{text: "B", values: map[string]ProofValue{"fontsize": {Value: "20:20", Known: true}}, sources: map[string]int{"fontsize": -1}},
			},
		},
		{
			name: "malformed assignment invalidates state until reset",
			text: `{\fs24\fsabc}A{\r}B`,
			texts: []interactionText{
				{text: "A", values: map[string]ProofValue{"fontsize": {Known: false}}},
				{text: "B", values: map[string]ProofValue{"fontsize": {Value: "20:20", Known: true}}, sources: map[string]int{"fontsize": -1}},
			},
		},
		{
			name: "mod position depth keeps a known depth",
			mod:  true,
			text: `{\pos(1,2,3)}A`,
			texts: []interactionText{
				{
					text: "A",
					values: map[string]ProofValue{
						"position": {Known: false},
						"z_depth":  {Value: "240:240", Known: true},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			profile := mustProfile(t, renderer.Libass)
			if tc.mod {
				profile = mustMod(t)
			}
			states := collectInteractionStates(t, tc.text, profile, styles)
			for _, want := range tc.texts {
				got, ok := states[want.text]
				if !ok {
					t.Fatalf("missing text run %q in %v", want.text, states)
				}
				if want.activeStyle != "" && got.ActiveStyle != want.activeStyle {
					t.Errorf("%q active style = %q, want %q", want.text, got.ActiveStyle, want.activeStyle)
				}
				for slot, value := range want.values {
					if got.Values[slot] != value {
						t.Errorf("%q %s = %#v, want %#v", want.text, slot, got.Values[slot], value)
					}
				}
				for slot, source := range want.sources {
					gotSource, present := got.Sources[slot]
					if source < 0 {
						if present {
							t.Errorf("%q %s source = %d, want style default", want.text, slot, gotSource)
						}
					} else if !present || gotSource != source {
						t.Errorf("%q %s source = %d (present=%v), want %d", want.text, slot, gotSource, present, source)
					}
				}
			}
		})
	}
}

type interactionText struct {
	text        string
	values      map[string]ProofValue
	sources     map[string]int
	activeStyle string
}

func collectInteractionStates(t *testing.T, text string, profile renderer.Profile, styles map[string]StyleState) map[string]StateSnapshot {
	t.Helper()
	states := make(map[string]StateSnapshot)
	EvaluateResolved(ass.ParseConcreteDialogue(text), profile, EvaluationOptions{
		Styles: styles, DialogueStyle: "Default", SkipNoEffectProofs: true,
		Observer: Observer{Text: func(value string, _ int, state StateView) {
			states[value] = state.Snapshot()
		}},
	})
	return states
}

// TestInteractionMatrixProofRevocation pins how a later special tag affects an
// earlier no-effect candidate. Malformed or renderer-dependent interactions
// revoke the proof; well-formed interactions keep it. A time-dependent
// transform is separate from a proof failure.
func TestInteractionMatrixProofRevocation(t *testing.T) {
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{"fontsize": "20", "bold": "0"}),
	}
	cases := []struct {
		name        string
		text        string
		wantRevoked bool
	}{
		{"unknown tag", `{\fs20\fs20}A{\mystery}B`, true},
		{"renderer-dependent clip", `{\fs20\fs20}A{\clip(1.5,0,10,10)}B`, true},
		{"malformed karaoke", `{\fs20\fs20}A{\k20junk}B`, true},
		{"missing named reset", `{\fs20\fs20}A{\rMissing}B`, true},
		{"malformed transform child", `{\fs20\fs20}A{\t(\fsabc)}B`, true},
		{"unknown transform child", `{\fs20\fs20}A{\t(\mystery)}B`, true},
		{"time-dependent transform", `{\fs20\fs20}A{\t(\bord4)}B`, false},
		{"bare reset", `{\fs20\fs20}A{\r}B`, false},
		{"karaoke", `{\fs20\fs20}A{\k20}B`, false},
		{"rectangular clip", `{\fs20\fs20}A{\clip(0,0,10,10)}B`, false},
		{"position", `{\fs20\fs20}A{\pos(1,2)}B`, false},
		{"drawing mode", `{\fs20\fs20}A{\p1}m 0 0 l 10 10{\p0}B`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			effects := evaluateLibass(ass.ParseConcreteDialogue(tc.text), EvaluationOptions{
				Styles: styles, DialogueStyle: "Default",
			}).NoEffects
			var found bool
			for _, effect := range effects {
				if effect.Tag.Name != "fs" || effect.Reason != SameValue {
					continue
				}
				found = true
				if effect.ProofRevoked != tc.wantRevoked {
					t.Fatalf("proof revoked=%v, want %v: %#v", effect.ProofRevoked, tc.wantRevoked, effect)
				}
			}
			if !found {
				t.Fatalf("missing no-effect candidate: %#v", effects)
			}
		})
	}
}
