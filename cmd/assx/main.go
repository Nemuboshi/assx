package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"assx/internal/ass"
	"assx/internal/lint"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("assx", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "pretty", "diagnostic output: pretty, plain, or json")
	fix := flags.Bool("fix", false, "apply safe fixes")
	unsafeFix := flags.Bool("unsafe-fix", false, "apply safe and unsafe fixes")
	checkFonts := flags.Bool("check-fonts", false, "check subtitle font availability and character coverage")
	fontDir := flags.String("font-dir", "", "font folder to scan instead of system fonts (requires --check-fonts)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: assx [--format pretty|plain|json] [--fix] [--unsafe-fix] [--check-fonts] [--font-dir DIR] file.ass")
	}
	if option := singleDashOption(args); option != "" {
		fmt.Fprintf(stderr, "Options must use the -- prefix: %s\n", option)
		return 2
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 {
		flags.Usage()
		return 2
	}
	if *format != "pretty" && *format != "plain" && *format != "json" {
		fmt.Fprintf(stderr, "Unknown output format %q. Use pretty, plain, or json.\n", *format)
		return 2
	}

	path := flags.Arg(0)
	started := time.Now()
	progress := startScanProgress(stderr, path, *format == "pretty")
	defer progress.stop()
	original, err := os.ReadFile(path)
	if err != nil {
		progress.stop()
		fmt.Fprintln(stderr, err)
		return 2
	}
	source, err := ass.DecodeSource(original)
	if err != nil {
		progress.stop()
		fmt.Fprintln(stderr, err)
		return 2
	}
	doc := ass.Parse(source.Text)
	var fontChecker *lint.FontChecker
	if *checkFonts {
		fontChecker, err = lint.NewFontChecker(*fontDir)
		if err != nil {
			progress.stop()
			fmt.Fprintf(stderr, "Failed to scan fonts: %v\n", err)
			return 2
		}
	}
	analyzeDoc := func(doc ass.Document) ([]lint.Diagnostic, error) {
		diagnostics := analyze(doc)
		if fontChecker != nil {
			fontDiagnostics, err := lint.AnalyzeFonts(doc, fontChecker)
			if err != nil {
				return nil, err
			}
			diagnostics = append(diagnostics, fontDiagnostics...)
			sort.SliceStable(diagnostics, func(i, j int) bool {
				if diagnostics[i].Line == diagnostics[j].Line {
					return diagnostics[i].Column < diagnostics[j].Column
				}
				return diagnostics[i].Line < diagnostics[j].Line
			})
		}
		return diagnostics, nil
	}
	diagnostics, err := analyzeDoc(doc)
	if err != nil {
		progress.stop()
		fmt.Fprintf(stderr, "Font check failed: %v\n", err)
		return 2
	}
	var appliedSafe, appliedUnsafe int
	if *fix || *unsafeFix {
		availableSafe, availableUnsafe, _ := lint.CountFixes(diagnostics)
		fixedText, count, err := lint.ApplyFixes(doc.Text, diagnostics, *unsafeFix)
		if err != nil {
			progress.stop()
			fmt.Fprintln(stderr, err)
			return 2
		}
		if count > 0 {
			if !bytes.Equal(source.Encode(doc.Text), original) {
				progress.stop()
				fmt.Fprintln(stderr, "Refusing to modify a file that cannot be losslessly round-tripped.")
				return 2
			}
			info, err := os.Stat(path)
			if err != nil {
				progress.stop()
				fmt.Fprintln(stderr, err)
				return 2
			}
			if err := writeAtomically(path, source.Encode(fixedText), info.Mode().Perm()); err != nil {
				progress.stop()
				fmt.Fprintln(stderr, err)
				return 2
			}
			appliedSafe = availableSafe
			if *unsafeFix {
				appliedUnsafe = availableUnsafe
			}
			doc = ass.Parse(fixedText)
			diagnostics, err = analyzeDoc(doc)
			if err != nil {
				progress.stop()
				fmt.Fprintf(stderr, "Font check failed: %v\n", err)
				return 2
			}
		}
	}
	progress.stop()
	elapsed := time.Since(started)

	for i := range diagnostics {
		diagnostics[i].File = path
	}
	safe, unsafe, unfixable := lint.CountFixes(diagnostics)
	summary := fmt.Sprintf("Fixes available: %d safe, %d unsafe, %d not auto-fixable.", safe, unsafe, unfixable)
	if appliedSafe+appliedUnsafe > 0 {
		summary = fmt.Sprintf("Applied fixes: %d safe, %d unsafe. %s", appliedSafe, appliedUnsafe, summary)
	}
	if *format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(diagnostics); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		fmt.Fprintln(stderr, summary)
	} else {
		color := *format == "pretty" && colorAllowed(stdout)
		renderHuman(stdout, path, diagnostics, elapsed, summary, color)
	}

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == lint.Error {
			return 1
		}
	}
	return 0
}

func singleDashOption(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--") {
			name, _, hasValue := strings.Cut(arg[2:], "=")
			if !hasValue && (name == "format" || name == "font-dir") {
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

type scanProgress struct {
	writer   io.Writer
	done     chan struct{}
	finished chan struct{}
	stopOnce sync.Once
}

func startScanProgress(writer io.Writer, path string, enabled bool) *scanProgress {
	progress := &scanProgress{writer: writer}
	if !enabled || !interactiveTerminal(writer) {
		return progress
	}
	progress.done = make(chan struct{})
	progress.finished = make(chan struct{})
	go func() {
		defer close(progress.finished)
		ticker := time.NewTicker(90 * time.Millisecond)
		defer ticker.Stop()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := 0
		draw := func() {
			icon := colorText(colorAllowed(writer), "36", frames[frame])
			fmt.Fprintf(writer, "\r\033[2K%s Scanning %s", icon, path)
			frame = (frame + 1) % len(frames)
		}
		draw()
		for {
			select {
			case <-progress.done:
				return
			case <-ticker.C:
				draw()
			}
		}
	}()
	return progress
}

func (p *scanProgress) stop() {
	if p.done == nil {
		return
	}
	p.stopOnce.Do(func() {
		close(p.done)
		<-p.finished
		fmt.Fprint(p.writer, "\r\033[2K")
	})
}

func interactiveTerminal(writer io.Writer) bool {
	return isTerminal(writer) && os.Getenv("TERM") != "dumb"
}

func colorAllowed(writer io.Writer) bool {
	return isTerminal(writer) && os.Getenv("TERM") != "dumb" && os.Getenv("NO_COLOR") == ""
}

func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func colorText(enabled bool, code, text string) string {
	if !enabled {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
}

func renderHuman(writer io.Writer, path string, diagnostics []lint.Diagnostic, elapsed time.Duration, fixSummary string, color bool) {
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
		fmt.Fprintln(writer, colorText(color, "32;1", "No issues found."))
	} else {
		for i, diagnostic := range ordered {
			label, code := strings.ToUpper(string(diagnostic.Severity)), "36"
			switch diagnostic.Severity {
			case lint.Error:
				code = "31;1"
			case lint.Warning:
				code = "33;1"
			}
			fmt.Fprintf(writer, "%s[%s] %s:%d:%d\n", colorText(color, code, label), colorText(color, "2", diagnostic.ID), path, diagnostic.Line, diagnostic.Column)
			fmt.Fprintf(writer, "    %s\n", diagnostic.Title)
			if diagnostic.Tag != "" {
				fmt.Fprintf(writer, "    tag: \\%s\n", diagnostic.Tag)
			}
			if diagnostic.Field != "" {
				fmt.Fprintf(writer, "    field: %s\n", diagnostic.Field)
			}
			if strings.HasPrefix(diagnostic.ID, "F") && diagnostic.Detail != "" {
				fmt.Fprintf(writer, "    %s\n", diagnostic.Detail)
			}
			if i+1 < len(ordered) {
				fmt.Fprintln(writer)
			}
		}
	}

	errors, warnings, lints := 0, 0, 0
	for _, diagnostic := range ordered {
		switch diagnostic.Severity {
		case lint.Error:
			errors++
		case lint.Warning:
			warnings++
		default:
			lints++
		}
	}
	fmt.Fprintf(writer, "\nChecked %s in %s. Summary: %d diagnostics (%d errors, %d warnings, %d lint findings).\n", path, formatDuration(elapsed), len(ordered), errors, warnings, lints)
	fmt.Fprintln(writer, fixSummary)
}

func formatDuration(elapsed time.Duration) string {
	if elapsed < time.Millisecond {
		return "<1ms"
	}
	return elapsed.Round(time.Millisecond).String()
}

func analyze(doc ass.Document) []lint.Diagnostic {
	return lint.AnalyzeDocument(doc)
}

func writeAtomically(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".assx-*.tmp")
	if err != nil {
		return err
	}
	tempPath := tmp.Name()
	defer os.Remove(tempPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}
