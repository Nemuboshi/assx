package lint

import "assx/internal/ass"

// AnalysisContext carries analysis-local infrastructure shared by lint stages.
type AnalysisContext struct {
	EditIndex *DialogueEditIndex
}

// DialogueEditIndex provides reusable source ownership information for one
// analysis pass. Concrete parsing results are tied to one document revision.
type DialogueEditIndex struct {
	dialogues map[int]ass.ConcreteDialogue
}

// NewDialogueEditIndex builds an analysis-local index of concrete dialogue
// ownership. The key is the document-local text offset.
func NewDialogueEditIndex(doc ass.Document) *DialogueEditIndex {
	index := &DialogueEditIndex{dialogues: make(map[int]ass.ConcreteDialogue, len(doc.Dialogues))}
	for _, dialogue := range doc.Dialogues {
		index.dialogues[dialogue.TextStart] = ass.ParseConcreteDialogue(dialogue.Text)
	}
	return index
}

// DialogueTree returns a previously indexed tree. Callers decide how to handle
// missing entries because SafeFix paths must not silently hide ownership misses.
func (i *DialogueEditIndex) DialogueTree(textStart int) (ass.ConcreteDialogue, bool) {
	tree, ok := i.dialogues[textStart]
	return tree, ok
}
