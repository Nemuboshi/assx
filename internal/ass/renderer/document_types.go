package renderer

// EventField keeps one Format-named event field with its raw source span.
// Value is the exact source text between separators; span offsets are byte
// offsets into Document.Text, so fixes and diagnostics keep raw data.
type EventField struct {
	Name  string // lowercased Format name
	Value string
	Start int
	End   int
}

// Dialogue is one event row as one renderer reads it.
type Dialogue struct {
	Text        string
	Style       string
	StyleColumn int
	Line        int
	LineStart   int
	TextStart   int
	Fields      []EventField
	// MissingFields marks a line that ran out of values before the Format
	// names did. libass discards such events (process_event_tail breaks on a
	// null token and returns failure), and xy-VSFilter throws in NextInt and
	// rejects the whole line.
	MissingFields bool
}

// Field returns the first event field whose Format name is name.
func (d Dialogue) Field(name string) (EventField, bool) {
	for _, field := range d.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return EventField{}, false
}

type HeaderField struct {
	Value      string
	Line       int
	ValueStart int
	ValueEnd   int
}

type StyleField struct {
	StyleName   string
	Name        string
	Value       string
	Line        int
	ValueStart  int
	ValueEnd    int
	ValueColumn int
}

// RawLine keeps one physical line with its byte offset and the section it
// appeared in, so document-level checks can reason about syntax libass drops.
type RawLine struct {
	Line    int
	Offset  int
	Content string
	Section string
}

// Document is one renderer-facing document view. It is not the source
// document: each profile can put different bytes in the same field.
type Document struct {
	Text             string
	Dialogues        []Dialogue
	StyleFields      []StyleField
	Headers          map[string]HeaderField
	Lines            []RawLine
	EventFormat      []string
	ScriptInfoLine   int
	ScriptInfoInsert int
	Newline          string
}
