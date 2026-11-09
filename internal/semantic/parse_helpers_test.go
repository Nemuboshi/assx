package semantic

import (
	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func defaultDocument(text string) renderer.Document {
	return renderer.DefaultDocument(ass.ParseSource(text))
}
