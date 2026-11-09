package main

import (
	"encoding/json/v2"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/fontcheck"
	"assx/internal/fontreport"
)

func runFonts(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("assx fonts check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "pretty", "output: pretty, plain, or json")
	directory := flags.String("font-dir", "", "exclusive font directory (default: system directories)")
	forceASS := flags.Bool("force-ass", false, "analyze unrecognized ASS input explicitly")
	primary := flags.String("renderer", "libass", "primary renderer: libass, xy-vsfilter, or vsfiltermod")
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	fail := func(err error) int { _, _ = fmt.Fprintln(stderr, err); return 2 }
	if flags.NArg() != 1 {
		return fail(fmt.Errorf("usage: assx fonts check [options] file.ass"))
	}
	if *format != "pretty" && *format != "plain" && *format != "json" {
		return fail(fmt.Errorf("unknown output format %q", *format))
	}
	explicitDirectory := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "font-dir" {
			explicitDirectory = true
		}
	})
	if explicitDirectory && strings.TrimSpace(*directory) == "" {
		return fail(fmt.Errorf("--font-dir requires a non-empty directory"))
	}
	kind, err := parseRenderer(*primary)
	if err != nil {
		return fail(err)
	}
	profile, err := renderer.New(kind)
	if err != nil {
		return fail(err)
	}
	path := flags.Arg(0)
	data, err := os.ReadFile(path)
	if err != nil {
		return fail(err)
	}
	source, err := ass.DecodeSource(data)
	if err != nil {
		return fail(err)
	}
	parsed := ass.ParseSource(source.Text)
	recognition := ass.RecognizeParsed(parsed)
	if !*forceASS {
		switch recognition.Class {
		case ass.Rejected:
			return fail(fmt.Errorf("input is not recognized as ASS/SSA; no font check was run. Use --force-ass to override input recognition"))
		case ass.Tentative:
			_, _ = fmt.Fprintln(stderr, "Input has tentative ASS/SSA structure; font analysis may be incomplete.")
		}
	}
	catalog, err := fontcheck.NewCatalog(fontcheck.Source{Directory: *directory})
	if err != nil {
		return fail(err)
	}
	if len(catalog.Issues) > 0 {
		return fail(fmt.Errorf("font catalog is incomplete: %s: %s (%d unreadable entries)", catalog.Issues[0].Path, catalog.Issues[0].Detail, len(catalog.Issues)))
	}
	usages := fontcheck.ExtractUsages(parsed, profile)
	findings, err := fontcheck.AnalyzeUsages(usages, catalog)
	if err != nil {
		return fail(err)
	}
	result := fontreport.Build(path, profile, catalog.Source, findings, usages)
	switch *format {
	case "json":
		err = json.MarshalWrite(stdout, result)
		if err == nil {
			_, err = io.WriteString(stdout, "\n")
		}
	case "plain":
		err = fontreport.Plain(stdout, result)
	default:
		err = fontreport.Pretty(stdout, result, source.Text, colorAllowed(stdout), outputWidth(stdout))
	}
	if err != nil {
		return fail(err)
	}
	return 0
}
