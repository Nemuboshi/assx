package lintreport

import (
	"assx/internal/lint"
	"assx/internal/report"
)

// Plain copies display fields without source parsing or proof work.
func Plain(diagnostics []lint.Diagnostic) []report.Diagnostic {
	views := make([]report.Diagnostic, len(diagnostics))
	for i := range diagnostics {
		views[i] = diagnosticView(&diagnostics[i])
	}
	return views
}

func diagnosticView(d *lint.Diagnostic) report.Diagnostic {
	return report.Diagnostic{
		Line: d.Line, Column: d.Column, ID: d.ID, Severity: string(d.Severity),
		Title: d.Title, Description: d.Description, Detail: d.Detail,
		Fix: d.Fix, Tag: d.Tag, Field: d.Field, Renderer: d.Renderer,
		Sources: append([]string(nil), d.Sources...),
	}
}
