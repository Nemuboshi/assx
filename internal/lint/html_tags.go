package lint

import (
	"strings"

	"assx/internal/ass/renderer"
)

// htmlTagNames lists the names that ParseHtmlTag accepts in the pinned
// xy-VSFilter and VSFilterMod sources. The libass event-text path has no HTML
// branch, so only the VSFilter profiles need the barrier.
//
// Sources: xy-VSFilter 135a3015 src/subtitles/RTS.cpp, ParseHtmlTag;
// VSFilterMod 7a00567e src/subtitles/RTS.cpp, ParseHtmlTag.
var htmlTagNames = map[string]struct{}{
	"text": {}, "b": {}, "strong": {}, "i": {}, "em": {}, "u": {},
	"s": {}, "strike": {}, "del": {}, "font": {}, "k": {},
}

// rendererParsesHTML reports whether the pinned renderer reads HTML tags from
// dialogue text before it treats the characters as ordinary text.
func rendererParsesHTML(profile renderer.Profile) bool {
	return profile.Kind() == renderer.XYVSFilter || profile.Kind() == renderer.VSFilterMod
}

// htmlTagSequence returns the raw inner text of each HTML tag that a VSFilter
// consumes in source order. It follows the renderer scan: a matched {..} block
// is skipped, and the nearest '>' closes a '<' candidate. The proof already
// equates the surrounding text, so equal sequences mean that the edit leaves
// the HTML interpretation unchanged.
func htmlTagSequence(text string) []string {
	var sequence []string
	for i := 0; i < len(text); {
		switch text[i] {
		case '{':
			if end := strings.IndexByte(text[i+1:], '}'); end >= 0 {
				i += end + 2
				continue
			}
		case '<':
			if end := strings.IndexByte(text[i+1:], '>'); end >= 0 {
				inner := text[i+1 : i+1+end]
				if isHTMLTagInner(inner) {
					sequence = append(sequence, inner)
					i += end + 2
					continue
				}
			}
		}
		i++
	}
	return sequence
}

// isHTMLTagInner mirrors ParseHtmlTag: trim spaces and slashes, then take the
// name before the first space and compare it in lower case.
func isHTMLTagInner(inner string) bool {
	if strings.HasPrefix(inner, "!--") {
		return true
	}
	name := strings.Trim(inner, " /")
	if i := strings.IndexByte(name, ' '); i >= 0 {
		name = name[:i]
	}
	_, ok := htmlTagNames[strings.ToLower(name)]
	return ok
}
