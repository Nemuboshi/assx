package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// Every branch of consumeTag declares one effect set: proof barrier, state
// invalidation, liveness, and latch uncertainty. This table pins the effect
// set of each branch so a new branch cannot silently skip an effect.
func TestTagBranchEffects(t *testing.T) {
	cases := []struct {
		name, text, tag string
		mod             bool
		index           int
		want            SemanticUncertainty
		barrier         bool
		ignored         bool
		applied         bool
		known           bool
		// knownAfter is evaluated inside the tag callback, after the transition.
		knownAfter map[string]bool
		// textKnown is evaluated at the first text boundary.
		textKnown map[string]bool
	}{
		{
			name: "repeated slashes", text: `{\fs24\\fs20}A`, tag: "fs20", index: 1,
			want: UncertaintyUnresolved, barrier: true,
			knownAfter: map[string]bool{"fontsize": false},
		},
		{
			name: "unclosed resets state", text: `{\fs24\fs(2}A`, tag: "fs", index: 1,
			want: UncertaintyMalformed, barrier: true,
			knownAfter: map[string]bool{"fontsize": false},
		},
		{
			name: "unclosed clip resets clip only", text: `{\fs24\clip(0,0,10,10)\clip(1,2,3}A`, tag: "clip", index: 2,
			want: UncertaintyMalformed, barrier: true,
			knownAfter: map[string]bool{"fontsize": true, "clip_rect": false, "clip_vector": false},
		},
		{
			name: "unknown tag", text: `{\fs24\mystery}A`, tag: "mystery", index: 1,
			want: UncertaintyUnsupported, barrier: true, ignored: true,
			knownAfter: map[string]bool{"fontsize": false},
		},
		{
			name: "rejected signature keeps state", text: `{\fs24\clip(1,2,3)}A`, tag: "clip", index: 1,
			want: UncertaintyMalformed, barrier: true, ignored: true,
			knownAfter: map[string]bool{"fontsize": true},
		},
		{
			name: "empty fade is known", text: `{\fs24\fade}A`, tag: "fade", index: 1,
			want: UncertaintyNone, known: true,
			knownAfter: map[string]bool{"fontsize": true},
		},
		{
			name: "unknown policy", text: `{\fs24\mover(1,2)}A`, tag: "mover", index: 1, mod: true,
			want: UncertaintyUnsupported, barrier: true,
			knownAfter: map[string]bool{"fontsize": false},
		},
		{
			name: "arity mismatch resets its slots", text: `{\fs24\fade(1)}A`, tag: "fade", index: 1,
			want: UncertaintyUnresolved, barrier: true,
			knownAfter: map[string]bool{"fontsize": true, "fade": false},
		},
		{
			name: "arity mismatch marks the latch uncertain",
			text: `{\fade(1)\fade(1,2)}A`, tag: "fade", index: 0,
			want: UncertaintyUnresolved, barrier: true,
			textKnown: map[string]bool{"fade": false},
		},
		{
			name: "first-wins owner keeps a rejected arity", text: `{\fade(1,2)\fade(1)}A`, tag: "fade", index: 1,
			want: UncertaintyUnresolved, ignored: true,
			knownAfter: map[string]bool{"fade": true},
		},
		{
			name: "folded assignment", text: `{\fs24\fs30}A`, tag: "fs", index: 1,
			want: UncertaintyNone, applied: true, known: true,
			knownAfter: map[string]bool{"fontsize": true},
		},
		{
			name: "supported transform child", text: `{\bord2\fs24\t(\bord2)}A`, tag: "bord", index: 3,
			want:       UncertaintyNone,
			knownAfter: map[string]bool{"fontsize": true, "border_x": true},
		},
		{
			name: "mod position depth", text: `{\fs24\pos(1,2,3)}A`, tag: "pos", index: 1, mod: true,
			want: UncertaintyRendererDependent, barrier: true, applied: true, known: false,
			knownAfter: map[string]bool{"fontsize": true, "position": false, "z_depth": true},
		},
		{
			name: "relative font size", text: `{\fs24\fs+10}A`, tag: "fs", index: 1,
			want: UncertaintyUnsupported, barrier: true,
			knownAfter: map[string]bool{"fontsize": false},
		},
		{
			name: "malformed clip value", text: `{\fs24\clip(0,0,10,10)\clip(1.5,0,10,10)}A`, tag: "clip", index: 2,
			want: UncertaintyRendererDependent, barrier: true, applied: true,
			knownAfter: map[string]bool{"fontsize": true, "clip_rect": false},
		},
		{
			name: "folded transform child", text: `{\fs24\t(\fsabc)}A`, tag: "fs", index: 2,
			want: UncertaintyMalformed, barrier: true,
			knownAfter: map[string]bool{"fontsize": false},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			profile, err := renderer.Standard(renderer.Libass)
			if err != nil {
				t.Fatal(err)
			}
			if test.mod {
				profile = mustMod(t)
			}
			var event TagEvent
			after := map[string]bool{}
			found := false
			text := map[string]bool{}
			EvaluateResolved(ass.ParseConcreteDialogue(test.text), profile, EvaluationOptions{
				Observer: Observer{
					Text: func(_ string, _ int, view StateView) {
						for slot := range test.textKnown {
							text[slot] = view.Value(slot).Known
						}
					},
					Tag: func(got TagEvent, view StateView) {
						if got.Index != test.index {
							return
						}
						event, found = got, true
						for slot := range test.knownAfter {
							after[slot] = view.Value(slot).Known
						}
					},
				},
			})
			if !found {
				t.Fatalf("missing event for tag %q at index %d", test.tag, test.index)
			}
			if event.Tag.Name != test.tag {
				t.Fatalf("tag = %q, want %q", event.Tag.Name, test.tag)
			}
			if event.Uncertainty != test.want || event.Barrier != test.barrier {
				t.Errorf("uncertainty=%v barrier=%v, want %v/%v", event.Uncertainty, event.Barrier, test.want, test.barrier)
			}
			if event.Ignored != test.ignored || event.Applied != test.applied || event.Known != test.known {
				t.Errorf("ignored=%v applied=%v known=%v, want %v/%v/%v",
					event.Ignored, event.Applied, event.Known, test.ignored, test.applied, test.known)
			}
			for slot, want := range test.knownAfter {
				if after[slot] != want {
					t.Errorf("slot %q known=%v after the tag, want %v", slot, after[slot], want)
				}
			}
			for slot, want := range test.textKnown {
				if text[slot] != want {
					t.Errorf("slot %q known=%v at text, want %v", slot, text[slot], want)
				}
			}
		})
	}
}
