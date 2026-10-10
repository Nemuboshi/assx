package ass

import "strings"

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

// ParseDialogueText resolves concrete block framing into the historical
// default-dialect Tag view. Tag naming and arity remain in legacy_tags.go;
// downstream default lint/state behavior does not depend on the neutral lexer.
func ParseDialogueText(text string) DialogueText {
	tree := DialogueText{Source: text}
	walkConcreteFrames(text, func(kind ConcreteNodeKind, start, end int) {
		if kind != ConcreteBraced {
			tree.Nodes = append(tree.Nodes, DialogueNode{
				Kind: TextNode, Start: start, End: end, Text: text[start:end],
			})
			return
		}
		block := parseOverrideBlock(text, start, end-1)
		nodeKind := CommentNode
		if block.IsEmpty() || block.HasTags() {
			nodeKind = OverrideNode
		}
		tree.Nodes = append(tree.Nodes, DialogueNode{
			Kind: nodeKind, Start: start, End: end, Block: &block,
		})
	})
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
