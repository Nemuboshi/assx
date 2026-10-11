package lint

import "assx/internal/ass"

func noEffectRemovalEditConcrete(tree ass.ConcreteDialogue, tag ass.Tag, textStart int) TextEdit {
	candidate := [2]int{tag.Start, tag.End}
	for _, node := range tree.Nodes {
		block := node.Block
		if block == nil || candidate[0] < block.ContentSpan.Start || candidate[1] > block.ContentSpan.End {
			continue
		}
		count, onlyCandidate := 0, true
		for _, item := range block.Items {
			if item.Expression != nil {
				count++
				if item.Expression.Span.Start != candidate[0] || item.Expression.Span.End != candidate[1] {
					onlyCandidate = false
				}
				continue
			}
			for i := range item.Raw {
				switch item.Raw[i] {
				case ' ', '\t', '\r', '\n', '\f', '\v':
				default:
					onlyCandidate = false
				}
			}
		}
		if count == 1 && onlyCandidate {
			return TextEdit{Start: textStart + block.Span.Start, End: textStart + block.Span.End}
		}
	}
	return TextEdit{Start: textStart + tag.Start, End: textStart + tag.End}
}
