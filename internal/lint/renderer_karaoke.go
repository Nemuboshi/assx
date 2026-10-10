package lint

import (
	"strconv"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/semantic"
)

// resolvedKaraokeCursor observes P05's authoritative state machine. It never
// re-parses a duration or independently applies timing arithmetic.
//
// P05 can establish a known accumulated value for a form whose signature is
// merely inferred. Lint additionally requires a source-verified signature for
// every contributing syllable before presenting ASS030 as a definite finding.
type resolvedKaraokeCursor struct {
	value      semantic.StateValue
	saw        bool
	unverified bool
}

func (c *resolvedKaraokeCursor) onTag(event semantic.TagEvent, state semantic.StateView) {
	c.value = state.Value("karaoke_cursor")
	if event.Ignored || event.Match != renderer.Matched {
		return
	}
	switch event.Tag.Name {
	case "k", "K", "kf", "ko":
		c.saw = true
		if event.Signature != renderer.SignatureVerified {
			c.unverified = true
		}
	}
}

func (c *resolvedKaraokeCursor) onText(_ string, _ int, state semantic.StateView) {
	c.value = state.Value("karaoke_cursor")
}

func (c *resolvedKaraokeCursor) duration() (int64, bool) {
	if !c.saw || c.unverified || !c.value.Known {
		return 0, false
	}
	value, err := strconv.ParseInt(c.value.Value, 10, 64)
	return value, err == nil && value >= 0
}

// karaokeCursorResolved is the standalone event-field entry point. The full
// scoped document analyzer attaches the same observer to its existing P05
// evaluation, avoiding another state-engine pass per dialogue.
func karaokeCursorResolved(tree ass.ConcreteDialogue, profile renderer.Profile) (int64, bool) {
	var cursor resolvedKaraokeCursor
	semantic.EvaluateResolved(tree, profile, semantic.EvaluationOptions{
		SkipNoEffectProofs: true,
		Observer: semantic.Observer{
			Tag: cursor.onTag, Text: cursor.onText,
		},
	})
	return cursor.duration()
}
