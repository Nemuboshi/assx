package lint

import "assx/internal/ass"

// DialogueEditIndex provides reusable source ownership information for one
// analysis pass. It keeps concrete parsing results local to the analysis and
// avoids rebuilding ownership relationships in individual fix generators.
type DialogueEditIndex struct {
	dialogues []indexedDialogue
}

type indexedDialogue struct {
	start int
	tree  ass.ConcreteDialogue
}

// NewDialogueEditIndex builds an analysis-local index of concrete dialogue
// ownership. The index deliberately owns no persistent cache because concrete
// spans are tied to a single document revision.
func NewDialogueEditIndex(doc ass.Document) *DialogueEditIndex {
	index := &DialogueEditIndex{dialogues: make([]indexedDialogue, 0, len(doc.Dialogues))}
	for _, dialogue := range doc.Dialogues {
		index.dialogues = append(index.dialogues, indexedDialogue{start: dialogue.TextStart, tree: ass.ParseConcreteDialogue(dialogue.Text)})
	}
	return index
}

func (i *DialogueEditIndex) DialogueTree(textStart int, text string) ass.ConcreteDialogue {
	for _, dialogue := range i.dialogues {
		if dialogue.start == textStart {
			return dialogue.tree
		}
	}
	return ass.ParseConcreteDialogue(text)
}
