package ass

import (
	"sort"
	"strings"

	"assx/internal/ass/spec"
)

type DialogueNodeKind uint8

const (
	TextNode DialogueNodeKind = iota
	CommentNode
	OverrideNode
)

type DialogueText struct {
	Source string
	Nodes  []DialogueNode
}

type DialogueNode struct {
	Kind  DialogueNodeKind
	Start int
	End   int
	Text  string
	Block *OverrideBlock
}

type OverrideBlock struct {
	Start        int
	End          int
	ContentStart int
	ContentEnd   int
	Raw          string
	Content      string
	Items        []BlockItem
}

func (block OverrideBlock) IsEmpty() bool {
	return strings.TrimSpace(block.Content) == ""
}

type BlockItemKind uint8

const (
	RawItem BlockItemKind = iota
	TagItem
)

type BlockItem struct {
	Kind  BlockItemKind
	Start int
	End   int
	Raw   string
	Tag   *Tag
}

type Tag struct {
	Name            string   `json:"tag"`
	Args            []string `json:"arguments,omitempty"`
	Column          int      `json:"column"`
	Paren           bool     `json:"-"`
	Start           int      `json:"-"`
	End             int      `json:"-"`
	InTransition    bool     `json:"-"`
	RepeatedSlashes int      `json:"-"`
	SlashStart      int      `json:"-"`
	Children        []Tag    `json:"-"`
	Raw             string   `json:"-"`
}

// RawArgument returns the argument text as written, including whitespace that
// can change a renderer's reset behavior.
func (tag Tag) RawArgument() (string, bool) {
	if len(tag.Args) > 1 {
		return "", false
	}
	if tag.Raw == "" {
		if len(tag.Args) == 0 {
			return "", true
		}
		return tag.Args[0], true
	}
	raw := strings.TrimLeft(strings.TrimPrefix(tag.Raw, "\\"), " \t")
	if !strings.HasPrefix(raw, tag.Name) {
		return "", false
	}
	raw = raw[len(tag.Name):]
	if tag.Paren {
		if len(raw) < 2 || raw[0] != '(' || raw[len(raw)-1] != ')' {
			return "", false
		}
		raw = raw[1 : len(raw)-1]
	}
	return raw, true
}

// IntegerArgument reads the leading signed decimal integer consumed by renderers.
// Overflow is unknown because libass and Windows integer conversion differ.
func (tag Tag) IntegerArgument() (int32, bool) {
	raw, known := tag.RawArgument()
	if !known {
		return 0, false
	}
	value := DecodeRendererInteger(raw)
	return int32(value.Integer), value.Status == ValueValid
}

type Token struct {
	Text  string
	Tag   *Tag
	Start int
}

// TokenView is a value passed synchronously to DialogueText.WalkTokens.
// Tag is meaningful when HasTag is true; its argument and child slices are
// borrowed from the tree and must not be mutated or retained by the callback.
type TokenView struct {
	Text   string
	Tag    Tag
	HasTag bool
	Start  int
}

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

func ParseDialogueText(text string) DialogueText {
	tree := DialogueText{Source: text}
	for offset := 0; offset < len(text); {
		open := strings.IndexByte(text[offset:], '{')
		if open < 0 {
			if offset < len(text) {
				tree.Nodes = append(tree.Nodes, DialogueNode{
					Kind: TextNode, Start: offset, End: len(text), Text: text[offset:],
				})
			}
			break
		}
		open += offset
		if open > offset {
			tree.Nodes = append(tree.Nodes, DialogueNode{
				Kind: TextNode, Start: offset, End: open, Text: text[offset:open],
			})
		}
		close := strings.IndexByte(text[open:], '}')
		if close < 0 {
			tree.Nodes = append(tree.Nodes, DialogueNode{
				Kind: TextNode, Start: open, End: len(text), Text: text[open:],
			})
			break
		}
		close += open
		block := parseOverrideBlock(text, open, close)
		kind := CommentNode
		if block.IsEmpty() || block.HasTags() {
			kind = OverrideNode
		}
		tree.Nodes = append(tree.Nodes, DialogueNode{
			Kind: kind, Start: open, End: close + 1, Block: &block,
		})
		offset = close + 1
	}
	return tree
}

// HasTags checks the already-parsed syntax tree without creating a token
// stream. Plain dialogue does not require override state evaluation.
func (tree DialogueText) HasTags() bool {
	for _, node := range tree.Nodes {
		if node.Kind == OverrideNode && node.Block != nil && node.Block.HasTags() {
			return true
		}
	}
	return false
}

func (tree DialogueText) Tokens() []Token {
	var tokens []Token
	tree.WalkTokens(func(view TokenView) bool {
		if view.HasTag {
			tag := view.Tag
			tokens = append(tokens, Token{Tag: &tag})
		} else {
			tokens = append(tokens, Token{Text: view.Text, Start: view.Start})
		}
		return true
	})
	return tokens
}

// WalkTokens emits text and tags in source order without materializing a
// flattened token slice. Returning false stops the traversal early.
func (tree DialogueText) WalkTokens(visit func(TokenView) bool) {
	stack := make([]*Tag, 0, 8)
	for _, node := range tree.Nodes {
		switch node.Kind {
		case TextNode:
			if node.Text != "" && !visit(TokenView{Text: node.Text, Start: node.Start}) {
				return
			}
		case OverrideNode:
			if node.Block == nil {
				continue
			}
			for _, item := range node.Block.Items {
				if item.Tag == nil {
					continue
				}
				stack = append(stack[:0], item.Tag)
				for len(stack) > 0 {
					last := len(stack) - 1
					current := stack[last]
					stack = stack[:last]
					if current.RepeatedSlashes > 0 {
						extra := Tag{
							Column:          current.SlashStart + 1,
							Start:           current.SlashStart,
							End:             current.Start,
							InTransition:    current.InTransition,
							RepeatedSlashes: current.RepeatedSlashes,
							SlashStart:      current.SlashStart,
						}
						if !visit(TokenView{Tag: extra, HasTag: true}) {
							return
						}
						normalized := *current
						normalized.RepeatedSlashes = 0
						if !visit(TokenView{Tag: normalized, HasTag: true}) {
							return
						}
					} else if !visit(TokenView{Tag: *current, HasTag: true}) {
						return
					}
					for i := len(current.Children) - 1; i >= 0; i-- {
						stack = append(stack, &current.Children[i])
					}
				}
			}
		}
	}
}

func (block OverrideBlock) Tags() []Tag {
	var tags []Tag
	for _, item := range block.Items {
		if item.Tag != nil {
			tags = append(tags, *item.Tag)
		}
	}
	return tags
}

func (block OverrideBlock) HasTags() bool {
	for _, item := range block.Items {
		if item.Tag != nil {
			return true
		}
	}
	return false
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
	for k < len(block) && block[k] != '(' && block[k] != '\\' && !(inTransition && block[k] == ')') {
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
