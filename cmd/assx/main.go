package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"

	"assx/internal/analysis"
	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/lint"
	"assx/internal/lintreport"
	"assx/internal/lsp"
	"assx/internal/report/plain"
	"assx/internal/report/pretty"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "fonts":
			if len(args) < 2 || args[1] != "check" {
				_, _ = fmt.Fprintln(stderr, "Usage: assx fonts check [options] file.ass")
				return 2
			}
			return runFonts(args[2:], stdout, stderr)
		case "lsp":
			return runLSP(args[1:], os.Stdin, stdout, stderr)
		case "lint":
			args = args[1:]
		}
	}
	flags := flag.NewFlagSet("assx", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "pretty", "diagnostic output: pretty, plain, or json")
	explain := flags.Bool("explain", false, "include expanded rule explanations and evidence in pretty output")
	fix := flags.Bool("fix", false, "apply safe fixes")
	unsafeFix := flags.Bool("unsafe-fix", false, "apply safe and unsafe fixes")
	forceASS := flags.Bool("force-ass", false, "analyze unrecognized ASS input explicitly")
	rendererNames := flags.String("renderer", "libass,xy-vsfilter", "renderers in priority order: the first is primary, the rest are compatibility and proof targets")
	// Usage and fallback diagnostics on stderr are best-effort: their write
	// failures cannot be reported through the same failing stream.
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: assx [lint] [--format pretty|plain|json] [--renderer=RENDERERS] [--explain] [--fix] [--unsafe-fix] [--force-ass] file.ass")
	}
	if err := flags.Parse(args); err != nil {
		if strings.Contains(err.Error(), "flag provided but not defined: -check-fonts") || strings.Contains(err.Error(), "flag provided but not defined: -font-dir") {
			_, _ = fmt.Fprintln(stderr, "Font checks moved to: assx fonts check [--font-dir DIR] file.ass")
		}
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
		_, _ = fmt.Fprintf(stderr, "Unknown output format %q. Use pretty, plain, or json.\n", *format)
		return 2
	}

	targets, err := parseAnalysisTargets(*rendererNames)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
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
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
		readPath = resolved
	}
	original, err := os.ReadFile(readPath)
	if err != nil {
		progress.stop()
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	source, err := ass.DecodeSource(original)
	if err != nil {
		progress.stop()
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	parsed := ass.ParseSource(source.Text)
	recognition := ass.RecognizeParsed(parsed)
	if !*forceASS {
		switch recognition.Class {
		case ass.Rejected:
			progress.stop()
			_, _ = fmt.Fprintln(stderr, "Input is not recognized as ASS/SSA; no lint or fixes were run. Use --force-ass to override input recognition.")
			return 2
		case ass.Tentative:
			_, _ = fmt.Fprintln(stderr, "Input has tentative ASS/SSA structure; analysis is allowed, but fixes will not be written. Use --force-ass to override input recognition.")
		}
	}
	snapshot, err := analysis.NewSnapshot(path, source.Text, nil)
	if err != nil {
		progress.stop()
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	// A CLI target selection is always explicit. Keep an empty compatibility
	// set non-nil so it never reads as the default-target sentinel.
	if targets.Compatibility == nil {
		targets.Compatibility = []renderer.Profile{}
	}
	options := analysis.Options{Targets: targets}
	ctx := context.Background()
	analyzeSnapshot := func(snapshot analysis.Snapshot, parsed ass.Parsed) (*analysis.Result, []lint.Diagnostic, error) {
		result, err := analysis.Analyze(ctx, snapshot, parsed, options)
		if err != nil {
			return nil, nil, err
		}
		diagnostics, err := result.Diagnostics(ctx)
		if err != nil {
			return nil, nil, err
		}
		return result, diagnostics, nil
	}
	result, diagnostics, err := analyzeSnapshot(snapshot, parsed)
	if err != nil {
		progress.stop()
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	var appliedSafe, appliedUnsafe int
	if (*fix || *unsafeFix) && (*forceASS || recognition.Class == ass.Recognized) {
		merged := diagnostics
		availableSafe, availableUnsafe, _ := lint.CountFixes(merged)
		fixedText, count, err := result.ApplyFixes(ctx, merged, *unsafeFix)
		if err != nil {
			progress.stop()
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
		if count > 0 {
			if !bytes.Equal(source.Encode(snapshot.Text()), original) {
				progress.stop()
				_, _ = fmt.Fprintln(stderr, "Refusing to modify a file that cannot be losslessly round-tripped.")
				return 2
			}
			info, err := os.Stat(readPath)
			if err != nil {
				progress.stop()
				_, _ = fmt.Fprintln(stderr, err)
				return 2
			}
			if err := writeAtomically(readPath, source.Encode(fixedText), info.Mode().Perm()); err != nil {
				progress.stop()
				_, _ = fmt.Fprintln(stderr, err)
				return 2
			}
			appliedSafe = availableSafe
			if *unsafeFix {
				appliedUnsafe = availableUnsafe
			}
			snapshot, err = analysis.NewSnapshot(path, fixedText, nil)
			if err != nil {
				progress.stop()
				_, _ = fmt.Fprintln(stderr, err)
				return 2
			}
			parsed = ass.ParseSource(fixedText)
			result, diagnostics, err = analyzeSnapshot(snapshot, parsed)
			if err != nil {
				progress.stop()
				_, _ = fmt.Fprintln(stderr, err)
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
	switch *format {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(diagnostics); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
		_, _ = fmt.Fprintln(stderr, summary)
	case "plain":
		if err := plain.Render(stdout, path, lintreport.Plain(diagnostics), elapsed, summary); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
	default:
		view := lintreport.Build(path, diagnostics, result.PrimaryView().Document)
		view.Explain = *explain
		view.Applied.Safe, view.Applied.Unsafe = appliedSafe, appliedUnsafe
		if err := pretty.Render(stdout, view, snapshot.Text(), colorAllowed(stdout), outputWidth(stdout)); err != nil {
			_, _ = fmt.Fprintln(stderr, err)
			return 2
		}
	}

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == lint.Error {
			return 1
		}
	}
	return 0
}

func runLSP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 0 && (len(args) != 1 || args[0] != "--stdio") {
		_, _ = fmt.Fprintln(stderr, "Usage: assx lsp [--stdio]")
		return 2
	}
	server := lsp.Server{Log: stderr}
	if err := server.Run(context.Background(), stdin, stdout); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func outputWidth(writer io.Writer) int {
	if file, ok := terminalFile(writer); ok {
		if width, _, err := term.GetSize(file.Fd()); err == nil && width > 0 {
			return width
		}
	}
	return 80
}

type scanProgress struct {
	writer   io.Writer
	done     chan struct{}
	finished chan struct{}
	stopOnce sync.Once
	ansi     bool
}

func startScanProgress(writer io.Writer, path string, enabled bool) *scanProgress {
	progress := &scanProgress{writer: writer}
	if !enabled || !interactiveTerminal(writer) {
		return progress
	}
	progress.ansi = ansiSupported(writer)
	progress.done = make(chan struct{})
	progress.finished = make(chan struct{})
	go func() {
		defer close(progress.finished)
		ticker := time.NewTicker(90 * time.Millisecond)
		defer ticker.Stop()
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		frame := 0
		draw := func() {
			icon := frames[frame]
			if progress.ansi {
				icon = colorText(true, "36", icon)
			}
			// Transient progress is best-effort; report output is checked separately.
			width := outputWidth(writer)
			_, _ = fmt.Fprint(writer, scanRedraw(scanStatus(icon, path, width), width, progress.ansi))
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

func scanStatus(icon, path string, width int) string {
	line := icon + " Scanning " + terminalText(path)
	if width > 1 && ansi.StringWidth(line) >= width {
		line = ansi.Truncate(line, width-1, "…")
	}
	return line
}

// scanRedraw returns one progress frame. Without ANSI support it pads with
// spaces so a plain carriage return overwrites the previous frame.
func scanRedraw(status string, width int, ansiTerm bool) string {
	if ansiTerm {
		return "\r\033[2K" + status
	}
	if pad := width - 1 - ansi.StringWidth(status); pad > 0 {
		status += strings.Repeat(" ", pad)
	}
	return "\r" + status
}

func (p *scanProgress) stop() {
	if p.done == nil {
		return
	}
	p.stopOnce.Do(func() {
		close(p.done)
		<-p.finished
		if p.ansi {
			_, _ = fmt.Fprint(p.writer, "\r\033[2K")
		} else {
			_, _ = fmt.Fprint(p.writer, scanRedraw("", outputWidth(p.writer), false), "\r")
		}
	})
}

// ansiSupported reports whether the writer accepts ANSI escape sequences.
func ansiSupported(writer io.Writer) bool {
	file, ok := terminalFile(writer)
	return ok && enableVirtualTerminal(file)
}

func interactiveTerminal(writer io.Writer) bool {
	return isTerminal(writer) && os.Getenv("TERM") != "dumb"
}

func colorAllowed(writer io.Writer) bool {
	return reportColorAllowed(isTerminal(writer), os.Getenv("TERM"), os.Getenv("NO_COLOR")) &&
		ansiSupported(writer)
}

func reportColorAllowed(tty bool, term, noColor string) bool {
	return tty && term != "dumb" && noColor == ""
}

// terminalFile reports the character device behind writer, if there is one.
// It is the single place that decides what counts as a terminal.
func terminalFile(writer io.Writer) (*os.File, bool) {
	file, ok := writer.(*os.File)
	if !ok {
		return nil, false
	}
	info, err := file.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return nil, false
	}
	return file, true
}

func isTerminal(writer io.Writer) bool {
	_, ok := terminalFile(writer)
	return ok
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

func writeAtomically(path string, data []byte, mode os.FileMode) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".assx-*.tmp")
	if err != nil {
		return err
	}
	tempPath := tmp.Name()
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, tmp.Close())
		}
		if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, removeErr)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	closeErr := tmp.Close()
	closed = true
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tempPath, path)
}
