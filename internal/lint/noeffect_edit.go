package lint

import "assx/internal/ass"

func noEffectRemovalEdit(tree ass.DialogueText, tag ass.Tag, textStart int) TextEdit {
	for i := range tree.Nodes {
		node := &tree.Nodes[i]
		if node.Kind != ass.OverrideNode || node.Block == nil {
			continue
		}
		matchCount := 0
		onlyCandidate := true
		for _, item := range node.Block.Items {
			if item.Tag != nil {
				matchCount++
				if item.Tag.Start != tag.Start || item.Tag.End != tag.End {
					onlyCandidate = false
				}
				continue
			}
			for _, b := range item.Raw {
				switch b {
				case ' ', '\t', '\r', '\n', '\f', '\v':
				default:
					onlyCandidate = false
				}
			}
		}
		if matchCount != 1 {
			continue
		}
		if onlyCandidate {
			return TextEdit{Start: textStart + node.Block.Start, End: textStart + node.Block.End}
		}
	}
	return TextEdit{Start: textStart + tag.Start, End: textStart + tag.End}
}
