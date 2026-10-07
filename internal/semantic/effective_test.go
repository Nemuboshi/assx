package semantic

import (
	"testing"

	"assx/internal/ass"
)

func TestEvaluateDialogueNoEffectReasons(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantReason NoEffectReason
		wantTag    string
		wantCount  int
	}{
		{name: "same value", text: `{\fs20\fs20}A`, wantReason: SameValue, wantTag: "fs", wantCount: 1},
		{name: "overwritten before use", text: `{\fs10\fs20}A`, wantReason: OverwrittenBeforeUse, wantTag: "fs", wantCount: 1},
		{name: "first wins", text: `{\an7\an8}A`, wantReason: FirstWinsIgnored, wantTag: "an", wantCount: 1},
		{name: "reset before use", text: `{\fs20\r}A`, wantReason: ResetBeforeUse, wantTag: "fs", wantCount: 1},
		{name: "multi slot same value", text: `{\xbord2\ybord2\bord2}A`, wantReason: SameValue, wantTag: "bord", wantCount: 1},
		{name: "used before reset", text: `{\fs20}A{\r}B`, wantCount: 0},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			effects := EvaluateDialogue(ass.ParseDialogueText(test.text))
			if len(effects) != test.wantCount {
				t.Fatalf("effects = %#v, want %d", effects, test.wantCount)
			}
			if test.wantCount == 0 {
				return
			}
			if effects[0].Reason != test.wantReason || effects[0].Tag.Name != test.wantTag {
				t.Fatalf("effect = %#v", effects[0])
			}
		})
	}
}

func TestEvaluateDialogueFirstWinsReportsOwner(t *testing.T) {
	effects := EvaluateDialogue(ass.ParseDialogueText(`{\an7\an8}A`))
	if len(effects) != 1 || effects[0].Reason != FirstWinsIgnored || effects[0].OwnerIndex != 0 {
		t.Fatalf("effects = %#v", effects)
	}
}

func TestEvaluateDialogueTransformNoEffect(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "empty still changes collision participation", text: `{\t()}A`, want: false},
		{name: "same target still changes collision participation", text: `{\bord2\t(\bord2)}A`, want: false},
		{name: "empty after position is redundant", text: `{\pos(100,100)\t()}A`, want: true},
		{name: "same target after position is redundant", text: `{\pos(100,100)\bord2\t(\bord2)}A`, want: true},
		{name: "second empty transform is redundant", text: `{\t()\t()}A`, want: true},
		{name: "different target", text: `{\bord2\t(\bord3)}A`, want: false},
		{name: "sequential transform matters", text: `{\bord2\t(\bord2\bord3)}A`, want: false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			effects := EvaluateDialogue(ass.ParseDialogueText(test.text))
			found := false
			for _, effect := range effects {
				if effect.Tag.Name == "t" && effect.Reason == TransitionNoEffect {
					found = true
				}
			}
			if found != test.want {
				t.Fatalf("effects = %#v, want transform finding=%v", effects, test.want)
			}
		})
	}
}

func TestTransitionInvalidatesEffectiveState(t *testing.T) {
	text := "{\\bord2\\t(\\bord4)\\bord2}A"
	effects := EvaluateDialogue(ass.ParseDialogueText(text))
	for _, effect := range effects {
		if effect.Tag.Name == "bord" {
			t.Fatalf("transform-dependent border was reported redundant: %#v", effects)
		}
	}
}
