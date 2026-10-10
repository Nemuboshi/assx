package ass

import (
	"sort"
	"strings"

	"assx/internal/ass/spec"
)

// This compatibility resolver preserves the default legacy tag dispatch
// pending per-renderer resolution (Issue #16 P04/P05). It must not be used
// by concrete syntax parsing: global names cannot define source syntax.

var tagNames []string

func init() {
	for name := range spec.TagSpecs {
		tagNames = append(tagNames, name)
	}
	sort.Slice(tagNames, func(i, j int) bool {
		if len(tagNames[i]) == len(tagNames[j]) {
			return tagNames[i] < tagNames[j]
		}
		return len(tagNames[i]) > len(tagNames[j])
	})
}

func parseOverrideBlock(text string, open, close int) OverrideBlock {
	contentStart := open + 1
	contentEnd := close
	content := text[contentStart:contentEnd]
	return OverrideBlock{
		Start:        open,
		End:          close + 1,
		ContentStart: contentStart,
		ContentEnd:   contentEnd,
		Raw:          text[open : close+1],
		Content:      content,
		Items:        parseBlockItems(content, contentStart, false),
	}
}

func parseBlockItems(block string, base int, inTransition bool) []BlockItem {
	var items []BlockItem
	cursor := 0
	for i := 0; i < len(block); {
		backslash := strings.IndexByte(block[i:], '\\')
		if backslash < 0 {
			break
		}
		backslash += i
		if backslash > cursor {
			items = appendRawItem(items, block[cursor:backslash], base+cursor)
		}

		runEnd := repeatedSlashEnd(block, backslash)
		tagStart := runEnd - 1
		if tagStart > backslash {
			items = appendRawItem(items, block[backslash:tagStart], base+backslash)
		}

		tag, end, ok := parseTag(block, base, tagStart, backslash, inTransition)
		if !ok {
			i = max(tagStart+1, end)
			cursor = i
			continue
		}
		items = append(items, BlockItem{
			Kind: TagItem, Start: tag.Start, End: tag.End, Tag: &tag,
		})
		i = end
		cursor = end
	}
	if cursor < len(block) {
		items = appendRawItem(items, block[cursor:], base+cursor)
	}
	return items
}

func appendRawItem(items []BlockItem, raw string, start int) []BlockItem {
	if raw == "" {
		return items
	}
	return append(items, BlockItem{
		Kind: RawItem, Start: start, End: start + len(raw), Raw: raw,
	})
}

type tagParseFrame struct {
	tag      Tag
	start    int
	argStart int
	depth    int
	args     []string
	children []Tag
}

func parseTag(block string, base, tagStart, slashStart int, inTransition bool) (Tag, int, bool) {
	frame, pos, ok, complete := beginTagParse(block, base, tagStart, slashStart, inTransition)
	if !ok {
		return Tag{}, pos, false
	}
	if complete {
		return frame.tag, pos, true
	}

	stack := []tagParseFrame{frame}
	for len(stack) > 0 {
		index := len(stack) - 1
		current := &stack[index]
		if pos >= len(block) {
			current.tag = finishTagParse(block, base, *current, pos, pos)
			stack = stack[:index]
			if len(stack) == 0 {
				return current.tag, pos, true
			}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, current.tag)
			continue
		}

		if current.tag.Name == "t" && current.depth == 0 && block[pos] == '\\' {
			runEnd := repeatedSlashEnd(block, pos)
			child, next, childOK, childComplete := beginTagParse(block, base, runEnd-1, pos, true)
			if !childOK {
				pos = max(runEnd, next)
				continue
			}
			if childComplete {
				current.children = append(current.children, child.tag)
				pos = next
				continue
			}
			stack = append(stack, child)
			pos = next
			continue
		}

		switch block[pos] {
		case '(':
			current.depth++
			pos++
		case ')':
			if current.depth > 0 {
				current.depth--
				pos++
				continue
			}
			current.tag = finishTagParse(block, base, *current, pos+1, pos)
			stack = stack[:index]
			pos++
			if len(stack) == 0 {
				return current.tag, pos, true
			}
			stack[len(stack)-1].children = append(stack[len(stack)-1].children, current.tag)
		case ',':
			if current.depth == 0 {
				current.args = append(current.args, strings.TrimSpace(block[current.argStart:pos]))
				current.argStart = pos + 1
				current.children = nil
			}
			pos++
		default:
			pos++
		}
	}
	return Tag{}, pos, false
}

func beginTagParse(block string, base, tagStart, slashStart int, inTransition bool) (tagParseFrame, int, bool, bool) {
	j := tagStart + 1
	for j < len(block) && isSpace(block[j]) {
		j++
	}
	k := j
	for k < len(block) && block[k] != '(' && block[k] != '\\' && (!inTransition || block[k] != ')') {
		k++
	}
	if k == j {
		return tagParseFrame{}, j, false, false
	}

	head := strings.TrimSpace(block[j:k])
	name := ""
	for _, candidate := range tagNames {
		if strings.HasPrefix(head, candidate) && recognizedTagPrefix(candidate, head[len(candidate):]) {
			name = candidate
			break
		}
	}
	if name == "" {
		if len(head) > 12 {
			head = head[:12]
		}
		tag := Tag{
			Name: head, Column: base + j + 1,
			Start: base + tagStart, End: base + k,
			SlashStart: base + slashStart, RepeatedSlashes: tagStart - slashStart,
			InTransition: inTransition, Raw: block[tagStart:k],
		}
		return tagParseFrame{tag: tag}, k, true, true
	}

	frame := tagParseFrame{
		tag: Tag{
			Name: name, Column: base + j + 1,
			Paren:      k < len(block) && block[k] == '(',
			Start:      base + tagStart,
			SlashStart: base + slashStart, RepeatedSlashes: tagStart - slashStart,
			InTransition: inTransition,
		},
		start: tagStart,
	}
	if !frame.tag.Paren {
		if rest := strings.TrimSpace(head[len(name):]); rest != "" {
			frame.tag.Args = []string{rest}
		}
		frame.tag.End = base + k
		frame.tag.Raw = block[tagStart:k]
		return frame, k, true, true
	}
	frame.argStart = k + 1
	return frame, k + 1, true, false
}

func finishTagParse(block string, base int, frame tagParseFrame, end, argEnd int) Tag {
	if argEnd > frame.argStart || len(frame.args) > 0 {
		frame.args = append(frame.args, strings.TrimSpace(block[frame.argStart:argEnd]))
	}
	frame.tag.Args = frame.args
	if frame.tag.Name == "t" {
		frame.tag.Children = frame.children
	}
	frame.tag.End = base + end
	frame.tag.Raw = block[frame.start:end]
	return frame.tag
}

func recognizedTagPrefix(name, value string) bool {
	if value == "" || spec.TagSpecs[name].Value == spec.FontNameValue || spec.TagSpecs[name].Behavior == spec.StyleReset {
		return true
	}
	first := value[0]
	isLetter := (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
	return !isLetter || len(name) > 1
}

func repeatedSlashEnd(block string, start int) int {
	end := start
	for end < len(block) && block[end] == '\\' {
		end++
	}
	return end
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == '\v'
}
