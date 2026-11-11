package lint

import (
	"slices"
	"strings"

	"assx/internal/ass/renderer"
	"assx/internal/edit"
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

// anyRendererParsesHTML reports whether one selected target reads HTML tags.
func anyRendererParsesHTML(profiles []renderer.Profile) bool {
	for _, profile := range profiles {
		if rendererParsesHTML(profile) {
			return true
		}
	}
	return false
}

// revokeCombinedHTMLChanges refuses every candidate in a dialogue when the
// complete candidate group for that dialogue changes the consumed tag
// sequence. Separate per-edit proofs are not sufficient here: two deletions
// can each expose no tag and still expose one together. The apply step checks
// the combined group again, so this pass keeps discovery and application
// consistent.
func revokeCombinedHTMLChanges(doc renderer.Document, diagnostics []Diagnostic, candidates []int, selected []FixEditProof, profiles []renderer.Profile) ([]int, []FixEditProof) {
	if len(doc.Dialogues) == 0 || !anyRendererParsesHTML(profiles) {
		return candidates, selected
	}
	byDialogue := dialogueEditsByIndex(doc, selected)
	if len(byDialogue) == 0 {
		return candidates, selected
	}
	var textEdits []FixEditProof
	for _, edits := range byDialogue {
		textEdits = append(textEdits, edits...)
	}
	changed, err := edit.Apply(doc.Text, proofTextEdits(textEdits))
	if err != nil {
		return candidates, selected
	}
	after, err := editedDialogueDocument(doc, changed, textEdits)
	if err != nil {
		return candidates, selected
	}
	changedLines := make(map[int]struct{})
	for i := range doc.Dialogues {
		if len(byDialogue[i]) == 0 {
			continue
		}
		if !slices.Equal(htmlTagSequence(doc.Dialogues[i].Text), htmlTagSequence(after.Dialogues[i].Text)) {
			changedLines[doc.Dialogues[i].Line] = struct{}{}
		}
	}
	if len(changedLines) == 0 {
		return candidates, selected
	}
	keptCandidates := make([]int, 0, len(candidates))
	var keptSelected []FixEditProof
	for _, index := range candidates {
		if _, changed := changedLines[diagnostics[index].Line]; changed {
			diagnostics[index].FixSafety = ""
			diagnostics[index].Edits = nil
			diagnostics[index].FixProof = nil
			diagnostics[index].FixProofRef = 0
			continue
		}
		keptCandidates = append(keptCandidates, index)
		for _, sourceEdit := range diagnostics[index].Edits {
			keptSelected = append(keptSelected, FixEditProof{Condition: diagnostics[index].Condition, Edit: sourceEdit})
		}
	}
	return keptCandidates, keptSelected
}
