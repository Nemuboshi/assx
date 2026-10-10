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
	"unicode"

	"github.com/charmbracelet/x/term"

	"assx/internal/ass"
	"assx/internal/lint"
	"assx/internal/report"
	"assx/internal/report/plain"
	"assx/internal/report/pretty"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("assx", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "pretty", "diagnostic output: pretty, plain, or json")
	explain := flags.Bool("explain", false, "include expanded rule explanations and evidence in pretty output")
	fix := flags.Bool("fix", false, "apply safe fixes")
	unsafeFix := flags.Bool("unsafe-fix", false, "apply safe and unsafe fixes")
	checkFonts := flags.Bool("check-fonts", false, "check subtitle font availability and character coverage")
	fontDir := flags.String("font-dir", "", "font folder to scan instead of system fonts (requires --check-fonts)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: assx [--format pretty|plain|json] [--explain] [--fix] [--unsafe-fix] [--check-fonts] [--font-dir DIR] file.ass")
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
	readPath := path
	if *fix || *unsafeFix {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			progress.stop()
			fmt.Fprintln(stderr, err)
			return 2
		}
		readPath = resolved
	}
	original, err := os.ReadFile(readPath)
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
		diagnostics := lint.AnalyzeDocument(doc)
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
			info, err := os.Stat(readPath)
			if err != nil {
				progress.stop()
				fmt.Fprintln(stderr, err)
				return 2
			}
			if err := writeAtomically(readPath, source.Encode(fixedText), info.Mode().Perm()); err != nil {
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
	} else if *format == "plain" {
		plain.Render(stdout, path, diagnostics, elapsed, summary)
	} else {
		view := report.Build(path, diagnostics, doc)
		view.Explain = *explain
		view.Applied.Safe, view.Applied.Unsafe = appliedSafe, appliedUnsafe
		pretty.Render(stdout, view, doc.Text, colorAllowed(stdout), outputWidth(stdout))
	}

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == lint.Error {
			return 1
		}
	}
	return 0
}

func outputWidth(writer io.Writer) int {
	if file, ok := writer.(*os.File); ok && isTerminal(writer) {
		if width, _, err := term.GetSize(file.Fd()); err == nil && width > 0 {
			return width
		}
	}
	return 80
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
			fmt.Fprintf(writer, "\r\033[2K%s Scanning %s", icon, terminalText(path))
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
	return reportColorAllowed(isTerminal(writer), os.Getenv("TERM"), os.Getenv("NO_COLOR"))
}

func reportColorAllowed(tty bool, term, noColor string) bool {
	return tty && term != "dumb" && noColor == ""
}

func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func terminalText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}

func colorText(enabled bool, code, text string) string {
	if !enabled {
		return text
	}
	return "\033[" + code + "m" + text + "\033[0m"
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
