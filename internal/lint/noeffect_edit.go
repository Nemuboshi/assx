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
		if blockHasOnlyWhitespaceBesidesTag(node.Block, tag) {
			return TextEdit{Start: textStart + node.Block.Start, End: textStart + node.Block.End}
		}
	}
	return TextEdit{Start: textStart + tag.Start, End: textStart + tag.End}
}

func blockHasOnlyWhitespaceBesidesTag(block *ass.OverrideBlock, tag ass.Tag) bool {
	for _, item := range block.Items {
		if item.Tag != nil {
			if item.Tag.Start != tag.Start || item.Tag.End != tag.End {
				return false
			}
			continue
		}
		for _, b := range item.Raw {
			switch b {
			case ' ', '\t', '\r', '\n', '\f', '\v':
			default:
				return false
			}
		}
	}
	return true
}
