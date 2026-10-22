package fontcheck

import (
	"fmt"
	"golang.org/x/image/font/sfnt"
	"os"
)

func (c *Catalog) missingRunes(context fontContext, text string) ([]rune, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	candidates := c.families[normalizeFontFamily(context.family)]
	if len(candidates) == 0 {
		return nil, false, nil
	}
	best := candidates[0]
	bestScore := styleDistance(best, context)
	for _, face := range candidates[1:] {
		if score := styleDistance(face, context); score < bestScore {
			best, bestScore = face, score
		}
	}
	fonts, ok := c.loaded[best.path]
	if !ok {
		data, err := os.ReadFile(best.path)
		if err != nil {
			return nil, true, fmt.Errorf("read font %q: %w", best.path, err)
		}
		collection, err := sfnt.ParseCollection(data)
		if err != nil {
			return nil, true, fmt.Errorf("parse font %q: %w", best.path, err)
		}
		fonts = make([]*sfnt.Font, collection.NumFonts())
		for i := range fonts {
			fonts[i], _ = collection.Font(i)
		}
		c.loaded[best.path] = fonts
	}
	if best.index >= len(fonts) || fonts[best.index] == nil {
		return nil, true, fmt.Errorf("font %q has no readable face %d", best.path, best.index)
	}

	selected := []*sfnt.Font{fonts[best.index]}
	if len(fonts) > 1 {
		seenFaces := map[int]bool{best.index: true}
		for _, face := range candidates {
			if face.path != best.path || styleDistance(face, context) != bestScore || seenFaces[face.index] || face.index >= len(fonts) || fonts[face.index] == nil {
				continue
			}
			selected = append(selected, fonts[face.index])
			seenFaces[face.index] = true
		}
	}
	seen := make(map[rune]bool)
	missing := make([]rune, 0)
	var buf sfnt.Buffer
	for _, r := range text {
		if r == '\n' || r == '\r' || seen[r] {
			continue
		}
		seen[r] = true
		found := false
		for _, font := range selected {
			glyph, err := font.GlyphIndex(&buf, r)
			if err == nil && glyph != 0 {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, r)
		}
	}
	return missing, true, nil
}

func styleDistance(face fontFace, context fontContext) int {
	score := 0
	if face.bold != context.bold {
		score++
	}
	if face.italic != context.italic {
		score++
	}
	return score
}
