package lint

import (
	"fmt"
	"strings"

	"assx/internal/ass"
)

// canonicalKeywords lists, per section, the exact keyword prefixes libass
// matches case-sensitively in ass.c (process_info_line, process_styles_line,
// process_events_line). VSFilter lowercases keywords first, so a line whose
// keyword matches only case-insensitively is honored there and silently
// dropped by libass.
var canonicalKeywords = map[string][]string{
	"script info": {
		"PlayResX:", "PlayResY:", "LayoutResX:", "LayoutResY:", "Timer:",
		"WrapStyle:", "ScaledBorderAndShadow:", "Kerning:", "YCbCr Matrix:",
		"Language:", "ScriptType:",
	},
	"v4+ styles": {"Format:", "Style:"},
	"v4 styles":  {"Format:", "Style:"},
	"events":     {"Format:", "Dialogue:", "Comment:"},
}

func AnalyzeKeywords(doc ass.Document) []Diagnostic {
	var diagnostics []Diagnostic
	for _, raw := range doc.Lines {
		keywords, sectionKnown := canonicalKeywords[raw.Section]
		if !sectionKnown {
			continue
		}
		trimmed := strings.TrimLeft(raw.Content, " \t")
		if trimmed == "" || trimmed[0] == '[' || trimmed[0] == ';' {
			continue
		}
		colon := strings.IndexByte(trimmed, ':')
		if colon < 0 {
			continue
		}
		// A space before the colon breaks both parsers: libass uses
		// strncmp on "Keyword:", and VSFilter compares the lowercased
		// text before the colon. Only a pure case difference diverges.
		prefix := trimmed[:colon]
		for _, keyword := range keywords {
			canonical := strings.TrimSuffix(keyword, ":")
			if strings.EqualFold(canonical, prefix) && canonical != prefix {
				rule := Rules[IssueKeywordCase]
				diagnostics = append(diagnostics, Diagnostic{
					ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
					Fix: rule.Fix, Line: raw.Line, Column: len(raw.Content) - len(trimmed) + 1,
					Field:   canonical,
					Detail:  fmt.Sprintf("libass only accepts the exact keyword %q; the %q spelling here is honored by VSFilter but dropped by libass.", keyword, prefix+":"),
					Sources: rule.Sources,
				})
			}
			if strings.EqualFold(canonical, prefix) {
				break
			}
		}
	}
	return diagnostics
}
