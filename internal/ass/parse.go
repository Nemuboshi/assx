package ass

// Parsed is one physical parse of a decoded source text. The concrete tree
// keeps the raw bytes and spans. Renderer views are derived from Concrete;
// this package does not own a default view.
type Parsed struct {
	Text     string
	Concrete ConcreteDocument
}

// ParseSource parses the physical structure once.
func ParseSource(text string) Parsed {
	return Parsed{Text: text, Concrete: ParseConcreteDocument(text)}
}

func IsStyleSection(section string) bool {
	return section == "v4+ styles" || section == "v4 styles" || section == "v4++ styles"
}

// HasFormatName reports whether a Format list contains the lowercased field
// name.
func HasFormatName(format []string, name string) bool {
	return hasFormatName(format, name)
}
