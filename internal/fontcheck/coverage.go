package fontcheck

import (
	"fmt"
	"os"

	"golang.org/x/image/font/sfnt"
)

func (c *Catalog) missingRunes(context fontContext, text string) ([]rune, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	faces := c.selectedFaces(context)
	if len(faces) == 0 {
		return nil, false, nil
	}
	path := faces[0].Path
	fonts, ok := c.loaded[path]
	if !ok {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, true, fmt.Errorf("read font %q: %w", path, err)
		}
		collection, err := sfnt.ParseCollection(data)
		if err != nil {
			return nil, true, fmt.Errorf("parse font %q: %w", path, err)
		}
		fonts = make([]*sfnt.Font, collection.NumFonts())
		for i := range fonts {
			fonts[i], _ = collection.Font(i)
		}
		c.loaded[path] = fonts
	}
	selected := make([]*sfnt.Font, 0, len(faces))
	for _, face := range faces {
		if face.Index < 0 || face.Index >= len(fonts) || fonts[face.Index] == nil {
			return nil, true, fmt.Errorf("font %q has no readable face %d", path, face.Index)
		}
		selected = append(selected, fonts[face.Index])
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
			if err != nil {
				return nil, true, fmt.Errorf("read glyph U+%04X in font %q: %w", r, path, err)
			}
			if glyph != 0 {
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
