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

type Token struct {
	Text  string
	Tag   *Tag
	Start int
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

func (tree DialogueText) Tokens() []Token {
	var tokens []Token
	for _, node := range tree.Nodes {
		switch node.Kind {
		case TextNode:
			if node.Text != "" {
				tokens = append(tokens, Token{Text: node.Text, Start: node.Start})
			}
		case OverrideNode:
			if node.Block == nil {
				continue
			}
			for _, item := range node.Block.Items {
				if item.Tag == nil {
					continue
				}
				appendFlatTagTokens(&tokens, *item.Tag)
			}
		}
	}
	return tokens
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

func appendFlatTagTokens(tokens *[]Token, tag Tag) {
	if tag.RepeatedSlashes > 0 {
		start := tag.SlashStart
		extra := Tag{
			Column:          start + 1,
			Start:           start,
			End:             tag.Start,
			InTransition:    tag.InTransition,
			RepeatedSlashes: tag.RepeatedSlashes,
			SlashStart:      start,
		}
		*tokens = append(*tokens, Token{Tag: &extra})
	}
	copyTag := tag
	copyTag.RepeatedSlashes = 0
	*tokens = append(*tokens, Token{Tag: &copyTag})
	for _, child := range tag.Children {
		appendFlatTagTokens(tokens, child)
	}
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

func parseTag(block string, base, tagStart, slashStart int, inTransition bool) (Tag, int, bool) {
	j := tagStart + 1
	for j < len(block) && isSpace(block[j]) {
		j++
	}
	k := j
	for k < len(block) && block[k] != '(' && block[k] != '\\' {
		k++
	}
	if k == j {
		return Tag{}, j, false
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
		name = head
		if len(name) > 12 {
			name = name[:12]
		}
		return Tag{
			Name: name, Column: base + j + 1,
			Start: base + tagStart, End: base + k,
			SlashStart:      base + slashStart,
			RepeatedSlashes: tagStart - slashStart,
			InTransition:    inTransition,
			Raw:             block[tagStart:k],
		}, k, true
	}

	var args []string
	paren := k < len(block) && block[k] == '('
	if paren {
		k++
		for {
			for k < len(block) && isSpace(block[k]) {
				k++
			}
			start := k
			for k < len(block) && block[k] != ',' && block[k] != '\\' && block[k] != ')' {
				k++
			}
			if k < len(block) && block[k] == ',' {
				args = append(args, strings.TrimSpace(block[start:k]))
				k++
				continue
			}
			if k < len(block) && block[k] == '\\' {
				if close := strings.IndexByte(block[k:], ')'); close >= 0 {
					k += close
				} else {
					k = len(block)
				}
			}
			if k > start || len(args) > 0 {
				args = append(args, strings.TrimSpace(block[start:k]))
			}
			if k < len(block) && block[k] == ')' {
				k++
			}
			break
		}
	} else if rest := strings.TrimSpace(head[len(name):]); rest != "" {
		args = []string{rest}
	}

	tag := Tag{
		Name: name, Args: args, Column: base + j + 1, Paren: paren,
		InTransition: inTransition, Start: base + tagStart, End: base + k,
		SlashStart: base + slashStart, RepeatedSlashes: tagStart - slashStart,
		Raw: block[tagStart:k],
	}
	if tag.Name == "t" && len(args) > 0 {
		argumentStart := strings.LastIndex(block[:k], args[len(args)-1])
		if argumentStart >= 0 {
			for _, item := range parseBlockItems(args[len(args)-1], base+argumentStart, true) {
				if item.Tag != nil {
					tag.Children = append(tag.Children, *item.Tag)
				}
			}
		}
	}
	return tag, k, true
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
