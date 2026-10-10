package pretty

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"assx/internal/lint"
	"assx/internal/report"
)

// Render prints a source-aware, color-independent diagnostic report.
func Render(w io.Writer, summary report.Summary, source string, color bool, width int) {
	lines := strings.Split(strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(source), "\n")
	for _, group := range summary.Groups {
		writeWrapped(w, "", "assx  "+group.File, width)
		writeWrapped(w, "", fmt.Sprintf("%d errors  |  %d warnings  |  %d suggestions", group.Errors, group.Warnings, group.Suggestions), width)
		fmt.Fprintln(w)
		if len(group.Diagnostics) == 0 {
			fmt.Fprintln(w, "No issues found.")
		}
		diagnostics := append([]report.Diagnostic(nil), group.Diagnostics...)
		sort.SliceStable(diagnostics, func(i, j int) bool {
			if diagnostics[i].Line != diagnostics[j].Line {
				return diagnostics[i].Line < diagnostics[j].Line
			}
			if diagnostics[i].SourceColumn != diagnostics[j].SourceColumn {
				return diagnostics[i].SourceColumn < diagnostics[j].SourceColumn
			}
			return diagnostics[i].ID < diagnostics[j].ID
		})
		lastSourceLine, lastFrameStart := 0, -1
		for i, d := range diagnostics {
			severity := clean(strings.ToLower(string(d.Severity)))
			if color {
				severity = severityStyle(d.Severity).Render(severity)
			}
			id := clean(d.ID)
			if color {
				id = lipgloss.NewStyle().Faint(true).Render(id)
			}
			location := fmt.Sprintf("%d:%d", d.Line, d.SourceColumn)
			fixedWidth := ansi.StringWidth(severity+"["+id+"]  ") + ansi.StringWidth("  "+location)
			title := ansi.Truncate(clean(d.Title), max(1, width-fixedWidth), "…")
			writeStyledWrapped(w, severity+"["+id+"]  "+title+"  "+location, width)
			if line, ok := sourceLine(lines, d.Line); ok {
				numberWidth := len(fmt.Sprint(d.Line))
				frameWidth := max(1, min(100, width-numberWidth-5))
				shown, marker, markWidth, frameStart := frame(line, d.SourceColumn-1, d.SourceWidth, frameWidth)
				if d.Line != lastSourceLine || frameStart != lastFrameStart {
					fmt.Fprintf(w, "  %*d | %s\n", numberWidth, d.Line, shown)
					lastSourceLine, lastFrameStart = d.Line, frameStart
				}
				if d.SourceColumn > 0 && d.SourceColumn-1 <= len(line) {
					fmt.Fprintf(w, "  %s | %s%s\n", strings.Repeat(" ", numberWidth), strings.Repeat(" ", marker), "^"+strings.Repeat("~", max(0, markWidth-1)))
				}
			}
			if summary.Explain {
				writeWrapped(w, "  ", d.Detail, width)
				if d.Description != "" {
					writeWrapped(w, "  ", d.Description, width)
				}
				for _, source := range d.Sources {
					writeWrapped(w, "  ", "Evidence: "+source, width)
				}
			} else if d.Detail != "" {
				writeWrapped(w, "  ", d.Detail, width)
			}
			switch d.Outcome {
			case report.FixSafe:
				writeWrapped(w, "  ", "Safe fix: "+d.Fix, width)
			case report.FixUnsafe:
				writeWrapped(w, "  ", "Unsafe fix (use --unsafe-fix): "+d.Fix, width)
			default:
				if d.Fix != "" {
					writeWrapped(w, "  ", "Fix: edit manually ("+d.Fix+")", width)
				} else {
					fmt.Fprintln(w, "  Fix: edit manually")
				}
			}
			if i+1 < len(group.Diagnostics) {
				fmt.Fprintln(w)
			}
		}
		footer := fmt.Sprintf("%d diagnostics", summary.Total)
		if summary.Applied.Total() > 0 {
			footer += fmt.Sprintf("  |  Applied fixes: %d safe, %d unsafe.", summary.Applied.Safe, summary.Applied.Unsafe)
		}
		fmt.Fprintln(w)
		writeWrapped(w, "", footer, width)
		writeWrapped(w, "", fmt.Sprintf("Fixes available: %d safe, %d unsafe, %d not auto-fixable.", summary.Remaining.Safe, summary.Remaining.Unsafe, summary.Remaining.Unfixable), width)
	}

}

func sourceLine(lines []string, number int) (string, bool) {
	if number < 1 || number > len(lines) {
		return "", false
	}
	return strings.TrimSuffix(lines[number-1], "\r"), true
}

func frame(s string, at, span, limit int) (string, int, int, int) {
	if at < 0 {
		at = 0
	}
	if at > len(s) {
		at = len(s)
	}
	if at < len(s) {
		for at > 0 && !utf8.RuneStart(s[at]) {
			at--
		}
	}
	if span < 1 {
		span = 1
	}
	endByte := min(len(s), at+span)
	if endByte < len(s) {
		for endByte > at && !utf8.RuneStart(s[endByte]) {
			endByte--
		}
	}
	markWidth := max(1, ansi.StringWidth(expand(s[at:endByte])))
	line := expand(s)
	cursor := ansi.StringWidth(expand(s[:at]))
	lineWidth := ansi.StringWidth(line)
	start := 0
	visible := limit
	if lineWidth > limit {
		visible = max(1, limit-2)
		start = max(0, cursor-visible/2)
		if start+visible > lineWidth {
			start = lineWidth - visible
		}
	}
	shown := ansi.Cut(line, start, start+visible)
	marker := cursor - start
	if start > 0 {
		shown = "…" + shown
		marker++
	}
	if start+visible < lineWidth {
		shown += "…"
	}
	markWidth = min(markWidth, max(1, ansi.StringWidth(shown)-marker))
	return shown, max(0, marker), markWidth, start
}

func writeStyledWrapped(w io.Writer, text string, width int) {
	for _, line := range strings.Split(ansi.Wrap(text, max(1, width), " /\\"), "\n") {
		fmt.Fprintln(w, line)
	}
}

func writeWrapped(w io.Writer, indent, text string, width int) {
	wrapped := ansi.Wrap(clean(text), max(1, width-ansi.StringWidth(indent)), " /\\")
	for _, line := range strings.Split(wrapped, "\n") {
		fmt.Fprintf(w, "%s%s\n", indent, line)
	}
}

func severityStyle(severity lint.Severity) lipgloss.Style {
	style := lipgloss.NewStyle().Bold(true)
	switch severity {
	case lint.Error:
		return style.Foreground(lipgloss.Color("9"))
	case lint.Warning:
		return style.Foreground(lipgloss.Color("3"))
	default:
		return style.Foreground(lipgloss.Color("6"))
	}
}

func expand(s string) string {
	var out strings.Builder
	cell := 0
	graphemes := uniseg.NewGraphemes(s)
	for graphemes.Next() {
		cluster := graphemes.Str()
		if cluster == "\\t" {
			n := 4 - cell%4
			out.WriteString(strings.Repeat(" ", n))
			cell += n
		} else if strings.IndexFunc(cluster, unicode.IsControl) >= 0 {
			out.WriteRune('�')
			cell++
		} else {
			out.WriteString(cluster)
			cell += ansi.StringWidth(cluster)
		}
	}
	return out.String()
}

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}
