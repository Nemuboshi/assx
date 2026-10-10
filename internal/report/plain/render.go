package plain

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"assx/internal/lint"
)

// Render preserves the existing plain diagnostic layout.
func Render(output io.Writer, path string, diagnostics []lint.Diagnostic, elapsed time.Duration, fixSummary string) error {
	// Formatting errors are retained by the buffer and returned by Flush.
	writer := bufio.NewWriter(output)
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
		_, _ = fmt.Fprintln(writer, "No issues found.")
	} else {
		for i, d := range ordered {
			_, _ = fmt.Fprintf(writer, "%s[%s] %s:%d:%d\n", strings.ToUpper(string(d.Severity)), d.ID, path, d.Line, d.Column)
			_, _ = fmt.Fprintf(writer, "    %s\n", d.Title)
			if d.Tag != "" {
				_, _ = fmt.Fprintf(writer, "    tag: \\%s\n", d.Tag)
			}
			if d.Field != "" {
				_, _ = fmt.Fprintf(writer, "    field: %s\n", d.Field)
			}
			if d.Detail != "" {
				_, _ = fmt.Fprintf(writer, "    %s\n", d.Detail)
			}
			if i+1 < len(ordered) {
				_, _ = fmt.Fprintln(writer)
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
	_, _ = fmt.Fprintf(writer, "\nChecked %s in %s. Summary: %d diagnostics (%d errors, %d warnings, %d suggestions).\n", path, formatDuration(elapsed), len(ordered), errors, warnings, suggestions)
	_, _ = fmt.Fprintln(writer, fixSummary)
	return writer.Flush()
}

func formatDuration(elapsed time.Duration) string {
	if elapsed < time.Millisecond {
		return "<1ms"
	}
	return elapsed.Round(time.Millisecond).String()
}
