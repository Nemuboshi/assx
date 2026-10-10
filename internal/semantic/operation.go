package semantic

import (
	"slices"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

// resolvedOperation is the only command/policy input accepted by the state
// engine. The legacy adapter exists solely for the frozen default CLI contract;
// renderer-scoped input is always resolved from the neutral CST.
type resolvedOperation struct {
	tag       ass.Tag
	policy    spec.TagSpec
	known     bool
	match     renderer.MatchStatus
	signature renderer.SignatureStatus
	shadowed  string
	// unclosed preserves a malformed parenthesized expression even when the
	// renderer resolver cannot assign a command name or signature.
	unclosed bool
}

type semanticToken struct {
	text   string
	start  int
	tag    ass.Tag
	hasTag bool
}

func (m *evaluator) operationFor(tag ass.Tag) resolvedOperation {
	if m.options.Profile != nil {
		return m.operations[tag.Start]
	}
	policy, known := spec.TagSpecs[tag.Name]
	return resolvedOperation{tag: tag, policy: policy, known: known, match: renderer.Matched}
}

func (m *evaluator) policyFor(tag ass.Tag) (spec.TagSpec, bool) {
	op := m.operationFor(tag)
	return op.policy, op.known
}

// resolveTokens retains visible text boundaries and the renderer's independent
// matching decisions. Children of a transform are resolved before evaluation,
// so parent no-effect checks cannot accidentally consult the global name table.
// A flat explicit stack avoids recursive parsing on adversarial nesting.
func resolveTokens(tree ass.ConcreteDialogue, profile renderer.Profile) ([]semanticToken, map[int]resolvedOperation) {
	source := tree.Source
	type entry struct {
		op     resolvedOperation
		parent int
	}
	type pending struct {
		expr   ass.ConcreteExpression
		parent int
		nested bool
	}
	var tokens []semanticToken
	var entries []entry

	for _, node := range tree.Nodes {
		if node.Block == nil {
			if node.Raw != "" {
				tokens = append(tokens, semanticToken{text: node.Raw, start: node.Span.Start})
			}
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression == nil {
				continue
			}
			stack := []pending{{expr: *item.Expression, parent: -1}}
			for len(stack) != 0 {
				last := len(stack) - 1
				current := stack[last]
				stack = stack[:last]
				result := profile.Resolve(current.expr, source)
				name := result.Name
				if name == "" {
					name = strings.TrimSpace(result.Head)
				}
				tag := ass.Tag{
					Name: name, Raw: result.Raw, Start: result.Source.Start,
					End: result.Source.End, SlashStart: current.expr.SlashSpan.Start,
					Column: current.expr.HeadSpan.Start + 1,
					Paren:  current.expr.Parenthesized, InTransition: current.nested,
				}
				if length := current.expr.SlashSpan.End - current.expr.SlashSpan.Start; length > 1 {
					tag.RepeatedSlashes = length - 1
				}
				for _, arg := range result.Args {
					tag.Args = append(tag.Args, strings.TrimSpace(arg.Raw))
				}
				index := len(entries)
				entries = append(entries, entry{
					op: resolvedOperation{
						tag: tag, policy: result.Policy, known: result.HasPolicy,
						match: result.Status, signature: result.Signature,
						shadowed: result.Shadowed, unclosed: current.expr.Parenthesized && !current.expr.Closed(),
					},
					parent: current.parent,
				})
				tokens = append(tokens, semanticToken{tag: tag, hasTag: true})
				if result.Status != renderer.Matched || result.Name != "t" || !current.expr.Parenthesized {
					continue
				}
				components := current.expr.Components(source)
				for i := len(components) - 1; i >= 0; i-- {
					for j := len(components[i].Items) - 1; j >= 0; j-- {
						if child := components[i].Items[j].Expression; child != nil {
							stack = append(stack, pending{expr: *child, parent: index, nested: true})
						}
					}
				}
			}
		}
	}
	// Populate the borrowed child views bottom-up. No independent parser or
	// interpreter reconstructs child tag names or argument forms.
	for i := len(entries) - 1; i >= 0; i-- {
		if parent := entries[i].parent; parent >= 0 {
			entries[parent].op.tag.Children = append(entries[parent].op.tag.Children, entries[i].op.tag)
		}
	}
	operations := make(map[int]resolvedOperation, len(entries))
	for i := range entries {
		slices.Reverse(entries[i].op.tag.Children)
		operations[entries[i].op.tag.Start] = entries[i].op
	}
	for i := range tokens {
		if tokens[i].hasTag {
			tokens[i].tag = operations[tokens[i].tag.Start].tag
		}
	}
	return tokens, operations
}
