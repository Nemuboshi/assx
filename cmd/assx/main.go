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

	"assx/internal/ass"
	"assx/internal/lint"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("assx", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "diagnostic output: text or json")
	fix := flags.Bool("fix", false, "apply safe fixes")
	unsafeFix := flags.Bool("unsafe-fix", false, "apply safe and unsafe fixes")
	checkFonts := flags.Bool("check-fonts", false, "check subtitle font availability and character coverage")
	fontDir := flags.String("font-dir", "", "font folder to scan instead of system fonts (requires --check-fonts)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: assx [--format text|json] [--fix] [--unsafe-fix] [--check-fonts] [--font-dir DIR] file.ass")
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
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "Unknown output format %q. Use text or json.\n", *format)
		return 2
	}

	path := flags.Arg(0)
	original, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	source, err := ass.DecodeSource(original)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	doc := ass.Parse(source.Text)
	var fontChecker *lint.FontChecker
	if *checkFonts {
		fontChecker, err = lint.NewFontChecker(*fontDir)
		if err != nil {
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
		fmt.Fprintf(stderr, "Font check failed: %v\n", err)
		return 2
	}
	var appliedSafe, appliedUnsafe int
	if *fix || *unsafeFix {
		availableSafe, availableUnsafe, _ := lint.CountFixes(diagnostics)
		fixedText, count, err := lint.ApplyFixes(doc.Text, diagnostics, *unsafeFix)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if count > 0 {
			if !bytes.Equal(source.Encode(doc.Text), original) {
				fmt.Fprintln(stderr, "Refusing to modify a file that cannot be losslessly round-tripped.")
				return 2
			}
			info, err := os.Stat(path)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			if err := writeAtomically(path, source.Encode(fixedText), info.Mode().Perm()); err != nil {
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
				fmt.Fprintf(stderr, "Font check failed: %v\n", err)
				return 2
			}
		}
	}

	for i := range diagnostics {
		diagnostics[i].File = path
	}
	switch *format {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(diagnostics); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	case "text":
		for _, d := range diagnostics {
			tag := ""
			if d.Tag != "" {
				tag = " \\" + d.Tag
			}
			if d.Field != "" {
				tag = " " + d.Field
			}
			if strings.HasPrefix(d.ID, "F") && d.Detail != "" {
				tag += " " + d.Detail
			}
			fmt.Fprintf(stdout, "%s:%d:%d: %s %s %s%s\n", path, d.Line, d.Column, strings.ToUpper(string(d.Severity)), d.ID, d.Title, tag)
		}
	}

	safe, unsafe, unfixable := lint.CountFixes(diagnostics)
	summary := fmt.Sprintf("Fixes available: %d safe, %d unsafe, %d not auto-fixable.", safe, unsafe, unfixable)
	if appliedSafe+appliedUnsafe > 0 {
		summary = fmt.Sprintf("Applied fixes: %d safe, %d unsafe. %s", appliedSafe, appliedUnsafe, summary)
	}
	if *format == "json" {
		fmt.Fprintln(stderr, summary)
	} else {
		fmt.Fprintln(stdout, summary)
	}

	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == lint.Error {
			return 1
		}
	}
	return 0
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
