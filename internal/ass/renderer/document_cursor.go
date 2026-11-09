package renderer

import (
	"strings"

	"assx/internal/ass"
)

// documentCursor holds the section and Format state while one pass scans the
// concrete lines. Each pass keeps its own interpretation rules and calls the
// cursor at the same boundaries.
type documentCursor struct {
	section     string
	styleFormat []string
	eventFormat []string
	scriptASS   bool
}

// enter applies one section header. A header line belongs to the previous
// section, so callers append their line record before they call enter.
func (c *documentCursor) enter(line ass.ConcreteLine) {
	if line.Header == nil {
		return
	}
	c.section = strings.ToLower(strings.TrimSpace(line.Header.RawName))
}

func (c *documentCursor) Section() string { return c.section }

func (c *documentCursor) resetStyleFormat() { c.styleFormat = nil }

func (c *documentCursor) setStyleFormat(body string, aliasActor bool) {
	c.styleFormat = ass.ParseFormat(body)
	if aliasActor {
		aliasActorColumn(c.styleFormat)
	}
}

func (c *documentCursor) setEventFormat(body string, aliasActor bool) {
	c.eventFormat = ass.ParseFormat(body)
	if aliasActor {
		aliasActorColumn(c.eventFormat)
	}
}

// styleFormatOrDefault returns the active Style Format. A pass supplies the
// dialect default for its own section rules.
func (c *documentCursor) styleFormatOrDefault(defaults func(string) []string) []string {
	if c.styleFormat != nil {
		return c.styleFormat
	}
	return defaults(c.section)
}

func (c *documentCursor) eventFormatOrDefault(defaults []string) []string {
	if c.eventFormat != nil {
		return c.eventFormat
	}
	return defaults
}

func aliasActorColumn(columns []string) {
	for i, column := range columns {
		if column == "actor" {
			columns[i] = "name"
		}
	}
}

// recordKey returns the lowercased key and the exact key without leading
// spaces. The two forms keep libass's exact-key rules separate from the
// case-insensitive VSFilter rules.
func recordKey(line ass.ConcreteLine) (key, exact string, ok bool) {
	if line.Record == nil {
		return "", "", false
	}
	key = strings.ToLower(strings.TrimSpace(line.Record.Key))
	exact = strings.TrimLeft(line.Record.Key, " \t")
	return key, exact, true
}
