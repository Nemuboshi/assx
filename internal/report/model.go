// Package report holds the renderer-independent view model for assx
// diagnostic output. It is assembled from lint results only; it never
// re-parses ASS, resolves semantics, or classifies fix safety. Presenters
// (plain and pretty) consume these views instead of touching lint internals directly.
package report

import (
	"assx/internal/lint"
)

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

// Diagnostic is the immutable presenter view of one finding. Coordinates are
// carried verbatim from lint.Diagnostic so the JSON contract and fix behavior
// stay the public truth; presenters normalize them before drawing frames.
type Diagnostic struct {
	Line         int
	Column       int
	SourceColumn int
	SourceWidth  int
	ID           string
	Severity     lint.Severity
	Title        string
	Description  string
	Detail       string
	Fix          string
	Tag          string
	Field        string
	FixSafety    lint.FixSafety
	Edits        []lint.TextEdit
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
	Groups    []Group
	Total     int
	Remaining FixCounts // fixes still available after this run
	Applied   FixCounts // fixes applied by this run (--fix / --unsafe-fix)
	Explain   bool
}
