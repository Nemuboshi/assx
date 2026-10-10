package lint

import (
	"math"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// karaokeCursorResolved follows independently dispatched operations. An
// unmodeled/conditional command cannot justify a timing claim, while a
// source-verified rejected shape never advances the cursor.
func karaokeCursorResolved(tree ass.ConcreteDialogue, profile renderer.Profile) (int64, bool) {
	const defaultSyllable = int64(1000)
	var cursor int64
	var saw, certain bool
	certain = true
	profile.WalkDialogue(tree, func(result renderer.Result, nested bool) bool {
		if result.Status != renderer.Matched || !result.Closed {
			certain = false
			return false
		}
		if result.Signature == renderer.SignatureRejected {
			return true
		}
		if !result.HasPolicy {
			certain = false
			return false
		}
		switch result.Name {
		case "kt":
			certain = false
			return false
		case "k", "K", "kf", "ko":
			if nested || result.Signature == renderer.SignatureUnknown {
				certain = false
				return false
			}
			saw = true
			duration := defaultSyllable
			switch len(result.Args) {
			case 0:
			case 1:
				value := ass.DecodeNumber(result.Args[0].Raw)
				if value.Status != ass.ValueValid || value.Number < 0 || value.Number > float64(math.MaxInt64/10) {
					certain = false
					return false
				}
				duration = int64(value.Number) * 10
			default:
				certain = false
				return false
			}
			if cursor > math.MaxInt64-duration {
				certain = false
				return false
			}
			cursor += duration
		}
		return true
	})
	return cursor, certain && saw
}
