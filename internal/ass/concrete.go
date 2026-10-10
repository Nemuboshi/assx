package ass

import "strings"

// ConcreteSpan is a half-open UTF-8 byte range in ConcreteDialogue.Source.
// It is intentionally independent of renderer-specific tag interpretation.
// The document parser uses the same half-open coordinate convention.
type ConcreteSpan struct {
	Start int
	End   int
}

type ConcreteNodeKind uint8

const (
	ConcreteText ConcreteNodeKind = iota
	ConcreteBraced
	ConcreteUnclosedBrace
)

// ConcreteDialogue is a lossless, renderer-neutral view of dialogue syntax.
// No tag dictionary, tag arity, or transform semantics participate in parsing.
// All strings are slices of Source; source spans always refer to Source.
type ConcreteDialogue struct {
	Source string
	Nodes  []ConcreteNode
}

type ConcreteNode struct {
	Kind  ConcreteNodeKind
	Span  ConcreteSpan
	Raw   string
	Block *ConcreteBlock // present only for matched braces
}

type ConcreteBlock struct {
	Span        ConcreteSpan
	ContentSpan ConcreteSpan
	Raw         string
	Content     string
	Items       []ConcreteItem
}

type ConcreteItemKind uint8

const (
	ConcreteRaw ConcreteItemKind = iota
	ConcreteCandidate
)

// ConcreteItem covers its entire source range, including otherwise invalid
// backslashes. Concatenating item.Raw reconstructs the block content exactly.
type ConcreteItem struct {
	Kind       ConcreteItemKind
	Span       ConcreteSpan
	Raw        string
	Expression *ConcreteExpression
}

// ConcreteExpression is a lexical candidate, NOT a tag. Head is the complete
// uninterpreted text after its slash run (e.g. "fsvp6", "fs20", "pos").
// Different renderers may resolve the same head to different tag identities.
// A parenthesized candidate keeps raw, optionally incomplete, argument text;
// comma offsets are syntax delimiters, not a renderer-specific arity decision.
type ConcreteExpression struct {
	Span          ConcreteSpan
	SlashSpan     ConcreteSpan
	HeadSpan      ConcreteSpan
	Head          string
	Parenthesized bool
	Open          int // absolute '(' byte offset, -1 when absent
	Close         int // absolute ')' byte offset, -1 when unterminated
	ContentSpan   ConcreteSpan
	Commas        []int // absolute byte offsets at the top parenthesis level
	Raw           string
}

func (e ConcreteExpression) Closed() bool {
	return !e.Parenthesized || e.Close >= 0
}

// HasCandidates reports whether the lossless tree contains override
// expressions. It is a cheap fast path for state-only consumers of plain text.
func (tree ConcreteDialogue) HasCandidates() bool {
	for _, node := range tree.Nodes {
		if node.Block == nil {
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression != nil {
				return true
			}
		}
	}
	return false
}

// ParseConcreteDialogue retains every source byte, even where syntax is
// incomplete. The first closing brace terminates a block, matching existing
// ASS dialogue framing; braced content is otherwise not interpreted here.
func ParseConcreteDialogue(source string) ConcreteDialogue {
	tree := ConcreteDialogue{Source: source}
	walkConcreteFrames(source, func(kind ConcreteNodeKind, start, end int) {
		if kind != ConcreteBraced {
			tree.appendText(start, end, kind)
			return
		}
		contentStart, contentEnd := start+1, end-1
		block := &ConcreteBlock{
			Span:        ConcreteSpan{start, end},
			ContentSpan: ConcreteSpan{contentStart, contentEnd},
			Raw:         source[start:end], Content: source[contentStart:contentEnd],
		}
		block.Items = parseConcreteItems(source, contentStart, contentEnd)
		tree.Nodes = append(tree.Nodes, ConcreteNode{
			Kind: ConcreteBraced, Span: block.Span, Raw: block.Raw, Block: block,
		})
	})
	return tree
}

// walkConcreteFrames shares only the renderer-independent brace framing.
// The callback selects which representation to construct. In particular the
// existing default Tag view need not allocate a second concrete tree.
func walkConcreteFrames(source string, visit func(ConcreteNodeKind, int, int)) {
	for offset := 0; offset < len(source); {
		relativeOpen := strings.IndexByte(source[offset:], '{')
		if relativeOpen < 0 {
			if offset < len(source) {
				visit(ConcreteText, offset, len(source))
			}
			break
		}
		open := offset + relativeOpen
		if open > offset {
			visit(ConcreteText, offset, open)
		}
		relativeClose := strings.IndexByte(source[open+1:], '}')
		if relativeClose < 0 {
			visit(ConcreteUnclosedBrace, open, len(source))
			break
		}
		close := open + 1 + relativeClose
		visit(ConcreteBraced, open, close+1)
		offset = close + 1
	}
}

func (tree *ConcreteDialogue) appendText(start, end int, kind ConcreteNodeKind) {
	if start == end {
		return
	}
	tree.Nodes = append(tree.Nodes, ConcreteNode{
		Kind: kind, Span: ConcreteSpan{start, end}, Raw: tree.Source[start:end],
	})
}

func parseConcreteItems(source string, start, end int) []ConcreteItem {
	var items []ConcreteItem
	cursor := start
	for cursor < end {
		slash := strings.IndexByte(source[cursor:end], '\\')
		if slash < 0 {
			items = appendConcreteRaw(items, source, cursor, end)
			break
		}
		slash += cursor
		items = appendConcreteRaw(items, source, cursor, slash)
		expr := parseConcreteExpression(source, slash, end)
		items = append(items, ConcreteItem{
			Kind: ConcreteCandidate, Span: expr.Span, Raw: expr.Raw, Expression: &expr,
		})
		cursor = expr.Span.End
	}
	return items
}

func appendConcreteRaw(items []ConcreteItem, source string, start, end int) []ConcreteItem {
	if start == end {
		return items
	}
	return append(items, ConcreteItem{
		Kind: ConcreteRaw, Span: ConcreteSpan{start, end}, Raw: source[start:end],
	})
}

func parseConcreteExpression(source string, start, limit int) ConcreteExpression {
	slashEnd := start
	for slashEnd < limit && source[slashEnd] == '\\' {
		slashEnd++
	}
	headEnd := slashEnd
	for headEnd < limit && source[headEnd] != '\\' && source[headEnd] != '(' {
		headEnd++
	}
	expr := ConcreteExpression{
		SlashSpan: ConcreteSpan{start, slashEnd},
		HeadSpan:  ConcreteSpan{slashEnd, headEnd},
		Head:      source[slashEnd:headEnd],
		Open:      -1, Close: -1,
	}
	end := headEnd
	if headEnd < limit && source[headEnd] == '(' {
		expr.Parenthesized = true
		expr.Open = headEnd
		depth := 1
		end = headEnd + 1
		for end < limit && depth > 0 {
			switch source[end] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					expr.Close = end
				}
			case ',':
				if depth == 1 {
					expr.Commas = append(expr.Commas, end)
				}
			}
			end++
		}
		contentEnd := end
		if expr.Close >= 0 {
			contentEnd = expr.Close
		}
		expr.ContentSpan = ConcreteSpan{headEnd + 1, contentEnd}
	}
	expr.Span = ConcreteSpan{start, end}
	expr.Raw = source[start:end]
	return expr
}
