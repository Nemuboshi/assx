package ass

// WalkConcreteExpressions visits lexical candidates, including nested components,
// in source order. Returning false stops traversal. No names are inferred.
func WalkConcreteExpressions(tree ConcreteDialogue, visit func(ConcreteExpression, bool) bool) {
	type entry struct {
		expression ConcreteExpression
		nested     bool
	}
	var stack []entry
	for _, node := range tree.Nodes {
		if node.Block == nil {
			continue
		}
		for _, item := range node.Block.Items {
			if item.Expression == nil {
				continue
			}
			stack = append(stack, entry{expression: *item.Expression})
			for len(stack) > 0 {
				last := len(stack) - 1
				current := stack[last]
				stack = stack[:last]
				if !visit(current.expression, current.nested) {
					return
				}
				components := current.expression.Components(tree.Source)
				for i := len(components) - 1; i >= 0; i-- {
					for j := len(components[i].Items) - 1; j >= 0; j-- {
						if child := components[i].Items[j].Expression; child != nil {
							stack = append(stack, entry{expression: *child, nested: true})
						}
					}
				}
			}
		}
	}
}
