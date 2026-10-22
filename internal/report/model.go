// Package report holds the renderer-independent view model for assx
// diagnostic output. It receives domain adapters; it never
// re-parses ASS, resolves semantics, or classifies fix safety. Presenters
// (plain and pretty) consume these views instead of touching lint internals directly.
package report

// FixOutcome is one of three trustworthy outcomes derived from per-diagnostic
// metadata, not from rule-level suggestions:
//   - FixNone:   no automatic edit is attached (manual fix guidance only).
//   - FixSafe:   concrete edits carry a validated SafeFix proof.
//   - FixUnsafe: concrete edits exist but are applied only with --unsafe-fix.
type FixOutcome uint8

const (
	FixNone FixOutcome = iota
	FixSafe
	FixUnsafe
)

// Diagnostic is a presenter view. Domain adapters supply source coordinates
// and any verified fix outcome. Domain JSON contracts remain separate.
type Diagnostic struct {
	Line         int
	Column       int
	SourceColumn int
	SourceWidth  int
	ID           string
	Severity     string
	Title        string
	Description  string
	Detail       string
	Fix          string
	Tag          string
	Renderer     string
	Field        string
	Sources      []string
	Outcome      FixOutcome
}

// Group is a run of diagnostics for one source file.
type Group struct {
	File         string
	Diagnostics  []Diagnostic
	Errors       int
	Warnings     int
	Suggestions  int
	FixAvailable FixCounts
}

// FixCounts aggregates fix outcomes over a set of diagnostics.
type FixCounts struct {
	Safe      int
	Unsafe    int
	Unfixable int
}

func (c FixCounts) Total() int { return c.Safe + c.Unsafe + c.Unfixable }

// Summary carries one run of results. Presenters must derive every visible
// count from structured data here; they never guess from messages.
type Summary struct {
	Heading   string
	Advisory  bool
	Notes     []string
	Groups    []Group
	Total     int
	Remaining FixCounts // fixes still available after this run
	Applied   FixCounts // fixes applied by this run (--fix / --unsafe-fix)
	Explain   bool
}
