package plain

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"assx/internal/lint"
)

// Render preserves the existing plain diagnostic layout.
func Render(writer io.Writer, path string, diagnostics []lint.Diagnostic, elapsed time.Duration, fixSummary string) {
	ordered := append([]lint.Diagnostic(nil), diagnostics...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Line == ordered[j].Line {
			if ordered[i].Column == ordered[j].Column {
				return ordered[i].ID < ordered[j].ID
			}
			return ordered[i].Column < ordered[j].Column
		}
		return ordered[i].Line < ordered[j].Line
	})
	if len(ordered) == 0 {
		fmt.Fprintln(writer, "No issues found.")
	} else {
		for i, d := range ordered {
			fmt.Fprintf(writer, "%s[%s] %s:%d:%d\n", strings.ToUpper(string(d.Severity)), d.ID, path, d.Line, d.Column)
			fmt.Fprintf(writer, "    %s\n", d.Title)
			if d.Tag != "" {
				fmt.Fprintf(writer, "    tag: \\%s\n", d.Tag)
			}
			if d.Field != "" {
				fmt.Fprintf(writer, "    field: %s\n", d.Field)
			}
			if d.Detail != "" {
				fmt.Fprintf(writer, "    %s\n", d.Detail)
			}
			if i+1 < len(ordered) {
				fmt.Fprintln(writer)
			}
		}
	}
	errors, warnings, suggestions := 0, 0, 0
	for _, d := range ordered {
		switch d.Severity {
		case lint.Error:
			errors++
		case lint.Warning:
			warnings++
		case lint.Suggestion:
			suggestions++
		}
	}
	fmt.Fprintf(writer, "\nChecked %s in %s. Summary: %d diagnostics (%d errors, %d warnings, %d suggestions).\n", path, formatDuration(elapsed), len(ordered), errors, warnings, suggestions)
	fmt.Fprintln(writer, fixSummary)
}

func formatDuration(elapsed time.Duration) string {
	if elapsed < time.Millisecond {
		return "<1ms"
	}
	return elapsed.Round(time.Millisecond).String()
}
