package ass

// ConcreteComponent is a raw top-level comma-delimited component inside a
// parenthesized lexical candidate. It does not denote a semantic argument.
// Items are lexical candidates and uninterpreted text within the component.
type ConcreteComponent struct {
	Span  ConcreteSpan
	Raw   string
	Items []ConcreteItem
}

// Components exposes parenthesized structure without classifying the outer
// expression (e.g. as a transform or clip) or any nested expression. Callers
// choose which renderer-specific arguments contain nested operations.
// The method is intentionally lazy: the default resolver does not allocate or
// traverse syntax it cannot yet interpret.
//
// Source must be the same source passed to ParseConcreteDialogue. Commas are
// already collected at depth one; commas in nested parentheses remain intact.
func (e ConcreteExpression) Components(source string) []ConcreteComponent {
	if !e.Parenthesized {
		return nil
	}
	components := make([]ConcreteComponent, 0, len(e.Commas)+1)
	start := e.ContentSpan.Start
	for _, comma := range e.Commas {
		components = append(components, concreteComponent(source, start, comma))
		start = comma + 1
	}
	components = append(components, concreteComponent(source, start, e.ContentSpan.End))
	return components
}

func concreteComponent(source string, start, end int) ConcreteComponent {
	return ConcreteComponent{
		Span:  ConcreteSpan{start, end},
		Raw:   source[start:end],
		Items: parseConcreteItems(source, start, end),
	}
}

// AtDocumentOffset maps a dialogue-relative source span into the decoded
// document's byte coordinates, which diagnostics and edits already use.
func (span ConcreteSpan) AtDocumentOffset(textStart int) ConcreteSpan {
	return ConcreteSpan{Start: textStart + span.Start, End: textStart + span.End}
}
