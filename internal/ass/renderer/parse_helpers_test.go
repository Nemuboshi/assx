package renderer

import "assx/internal/ass"

// Parse is a test-only shorthand for the default view.
func Parse(text string) Document {
	return DefaultDocument(ass.ParseSource(text))
}
