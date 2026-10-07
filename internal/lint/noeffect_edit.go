package lint

import "assx/internal/ass"

func noEffectRemovalEdit(tree ass.DialogueText, tag ass.Tag, textStart int) TextEdit {
	for i := range tree.Nodes {
		node := &tree.Nodes[i]
		if node.Kind != ass.OverrideNode || node.Block == nil {
			continue
		}
		tags := node.Block.Tags()
		if len(tags) != 1 || tags[0].Start != tag.Start || tags[0].End != tag.End {
			continue
		}
		if blockContainsOnlyCandidateTags(node.Block, map[[2]int]bool{[2]int{tag.Start, tag.End}: true}) {
			return TextEdit{Start: textStart + node.Block.Start, End: textStart + node.Block.End}
		}
	}
	return TextEdit{Start: textStart + tag.Start, End: textStart + tag.End}
}
