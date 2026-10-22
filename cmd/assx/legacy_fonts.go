package main

import (
	"fmt"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/fontcheck"
	"assx/internal/lint"
)

// legacyFontDiagnostics preserves the optional lint contract through F02 only.
func legacyFontDiagnostics(doc ass.Document, catalog *fontcheck.Catalog, profile renderer.Profile) ([]lint.Diagnostic, error) {
	findings, err := fontcheck.Analyze(doc, catalog, profile)
	if err != nil {
		return nil, err
	}
	var diagnostics []lint.Diagnostic
	for _, finding := range findings {
		usage := finding.Usages[0]
		id := lint.IssueFontMissing
		detail := fmt.Sprintf("Font family %q was not found in the scanned fonts.", usage.Request.Family)
		if finding.Kind == fontcheck.MissingCharacters {
			id = lint.IssueMissingGlyphs
			detail = fmt.Sprintf("Font family %q does not contain these characters: %q.", usage.Request.Family, finding.Missing)
		}
		rule := lint.Rules[id]
		diagnostics = append(diagnostics, lint.Diagnostic{
			ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, Line: usage.Line, Column: usage.Column, Detail: detail, Sources: rule.Sources,
			Renderer: lint.ProfileScope(profile),
		})
	}
	return diagnostics, nil
}
