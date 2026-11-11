package semantic

import (
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

// resolvedOperation is the only command/policy input accepted by the state
// engine. Input is always resolved from the neutral CST.
type resolvedOperation struct {
	tag       renderer.Tag
	policy    spec.TagSpec
	known     bool
	match     renderer.MatchStatus
	signature renderer.SignatureStatus
	shadowed  string
	citation  string
	// unclosed preserves a malformed parenthesized expression even when the
	// renderer resolver cannot assign a command name or signature.
	unclosed bool
}

// semanticToken is one step of the neutral dialogue program. A tag token
// carries only its source offsets; each profile resolves it through its own
// operation map. No renderer decision is stored here.
type semanticToken struct {
	text   string
	start  int
	tag    renderer.Tag
	hasTag bool
}

// program is one dialogue flattened once per concrete tree. Transform nesting
// is a child-index map, not a recursive tree. The token stream is neutral:
// every selected renderer walks the same sequence.
type program struct {
	tokens   []semanticToken
	children map[int][]int // parent tag start -> child tag starts, in source order
}

func (m *evaluator) operationFor(tag renderer.Tag) resolvedOperation {
	return m.operations[tag.Start]
}

func (m *evaluator) policyFor(tag renderer.Tag) (spec.TagSpec, bool) {
	op := m.operationFor(tag)
	return op.policy, op.known
}

// buildProgram flattens the CST once. Transform children are found from the
// syntax tree, not from a renderer result, so every profile receives the same
// token sequence.
func buildProgram(tree ass.ConcreteDialogue) *program {
	source := tree.Source
	built := &program{}
	type pending struct {
		expr      ass.ConcreteExpression
		parent    int
		nested    bool
		hasParent bool
	}
	for _, node := range tree.Nodes {
		if node.Block == nil {
			if node.Raw != "" {
				built.tokens = append(built.tokens, semanticToken{text: node.Raw, start: node.Span.Start})
			}
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression == nil {
				continue
			}
			stack := []pending{{expr: *item.Expression, parent: -1, nested: false}}
			for len(stack) != 0 {
				last := len(stack) - 1
				current := stack[last]
				stack = stack[:last]
				tag := renderer.Tag{
					Start: current.expr.Span.Start, End: current.expr.Span.End,
					Column: current.expr.HeadSpan.Start + 1,
					Paren:  current.expr.Parenthesized, InTransition: current.nested,
				}
				if length := current.expr.SlashSpan.End - current.expr.SlashSpan.Start; length > 1 {
					tag.RepeatedSlashes = length - 1
				}
				built.tokens = append(built.tokens, semanticToken{tag: tag, hasTag: true})
				if current.hasParent {
					if built.children == nil {
						built.children = make(map[int][]int)
					}
					built.children[current.parent] = append(built.children[current.parent], tag.Start)
				}
				if strings.TrimSpace(current.expr.Head) != "t" || !current.expr.Parenthesized {
					continue
				}
				components := current.expr.Components(source)
				for i := len(components) - 1; i >= 0; i-- {
					for j := len(components[i].Items) - 1; j >= 0; j-- {
						if child := components[i].Items[j].Expression; child != nil {
							stack = append(stack, pending{expr: *child, parent: tag.Start, nested: true, hasParent: true})
						}
					}
				}
			}
		}
	}
	return built
}

// resolvedEntry is one operation in renderer.Profile.WalkDialogue emission
// order: the renderer's own resolution result and its transform nesting flag.
// Prepared dialogues retain these entries so validation and compatibility
// never resolve the same expression twice.
type resolvedEntry struct {
	result renderer.Result
	nested bool
}

// resolveOperations resolves every expression in the neutral program under one
// profile. With collectEntries set, the second result replays
// renderer.Profile.WalkDialogue emission order.
func resolveOperations(tree ass.ConcreteDialogue, profile renderer.Profile, collectEntries bool) (map[int]resolvedOperation, []resolvedEntry) {
	source := tree.Source
	operations := make(map[int]resolvedOperation)
	var out []resolvedEntry
	if collectEntries {
		out = []resolvedEntry{}
	}
	type pending struct {
		expr   ass.ConcreteExpression
		nested bool
	}
	for _, node := range tree.Nodes {
		if node.Block == nil {
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression == nil {
				continue
			}
			stack := []pending{{expr: *item.Expression}}
			for len(stack) != 0 {
				last := len(stack) - 1
				current := stack[last]
				stack = stack[:last]
				result := profile.Resolve(current.expr, source)
				name := result.Name
				if name == "" {
					name = strings.TrimSpace(result.Head)
				}
				tag := renderer.Tag{
					Name: name, Raw: result.Raw, Start: result.Source.Start,
					End: result.Source.End, SlashStart: current.expr.SlashSpan.Start,
					Column: current.expr.HeadSpan.Start + 1,
					Paren:  current.expr.Parenthesized, InTransition: current.nested,
				}
				if length := current.expr.SlashSpan.End - current.expr.SlashSpan.Start; length > 1 {
					tag.RepeatedSlashes = length - 1
				}
				for _, arg := range result.Args {
					tag.Args = append(tag.Args, renderer.TrimArgumentSpace(arg.Raw))
				}
				operations[tag.Start] = resolvedOperation{
					tag: tag, policy: result.Policy, known: result.HasPolicy,
					match: result.Status, signature: result.Signature,
					shadowed: result.Shadowed, citation: result.Citation,
					unclosed: current.expr.Parenthesized && !current.expr.Closed(),
				}
				if collectEntries {
					out = append(out, resolvedEntry{result: result, nested: current.nested})
				}
				if strings.TrimSpace(current.expr.Head) != "t" || !current.expr.Parenthesized {
					continue
				}
				components := current.expr.Components(source)
				for i := len(components) - 1; i >= 0; i-- {
					for j := len(components[i].Items) - 1; j >= 0; j-- {
						if child := components[i].Items[j].Expression; child != nil {
							stack = append(stack, pending{expr: *child, nested: true})
						}
					}
				}
			}
		}
	}
	return operations, out
}
