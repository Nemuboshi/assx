package semantic

import (
	"testing"

	"assx/internal/ass"
)

// Once a renderer-dependent tag has been encountered, earlier no-effect
// proofs must not survive. The unknown may retroactively change whether
// removing those earlier tags is safe on all supported renderers.
func TestUnmodeledTagsRevokeEarlierNoEffectProofs(t *testing.T) {
	cases := []struct{ name, text string }{
		{"unknown in same block", `{\fs20\fs20\mystery}A`},
		{"unknown in later block", `{\fs20\fs20}A{\mystery}B`},
		{"unknown in transform", `{\fs20\fs20\t(\mystery)}A`},
		{"unknown in later transform", `{\fs20\fs20}A{\t(\mystery)}B`},
		{"VSFilterMod in same block", `{\fs20\fs20\1img}A`},
		{"VSFilterMod in later block", `{\fs20\fs20}A{\1img}B`},
		{"VSFilterMod in transform", `{\fs20\fs20\t(\1img)}A`},
		{"VSFilterMod in later transform", `{\fs20\fs20}A{\t(\1img)}B`},
		{"unknown numeric value", `{\fs20\fs20\fsabc}A`},
		{"unknown numeric inside transform", `{\fs20\fs20\t(\fsabc)}A`},
		{"unknown malformed arity", `{\fs20\fs20\pos(1)}A`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			effects := EvaluateDialogue(ass.ParseDialogueText(tc.text))
			if len(effects) == 0 {
				t.Fatal("expected a preserved but non-fixable diagnostic candidate")
			}
			for _, effect := range effects {
				if !effect.ProofRevoked {
					t.Fatalf("unmodeled operation left safe no-effect proof: %#v", effects)
				}
			}
		})
	}
}

func TestSupportedTagsStillAllowNoEffectProofs(t *testing.T) {
	for _, tc := range []string{
		`{\fs20\fs20}A`,
		`{\fs20\fs20\bord2}A`,
		`{\fs20\fs20\k20}A`,
		`{\fs20\fs20\clip(0,0,10,10)}A`,
		`{\fs20\fs20\pos(10,20)}A`,
	} {
		effects := EvaluateDialogue(ass.ParseDialogueText(tc))
		if len(effects) == 0 {
			t.Errorf("ordinary supported tag should keep no-effect proof: %q", tc)
		}
		for _, effect := range effects {
			if effect.ProofRevoked {
				t.Errorf("supported tag unexpectedly revoked proof for %q: %#v", tc, effect)
			}
		}
	}
}
