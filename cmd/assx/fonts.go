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
	primary := flags.String("renderer", "libass", "primary renderer: libass, xy-vsfilter, or vsfiltermod")
	modName := flags.String("mod-build", "unknown", "VSFilterMod capability: unknown, enabled, or disabled")
	luaName := flags.String("lua-build", "unknown", "Lua capability: unknown, enabled, or disabled")
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
	explicitDirectory, explicitBuild := false, false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "font-dir" {
			explicitDirectory = true
		}
		if f.Name == "mod-build" || f.Name == "lua-build" {
			explicitBuild = true
		}
	})
	if explicitDirectory && strings.TrimSpace(*directory) == "" {
		return fail(fmt.Errorf("--font-dir requires a non-empty directory"))
	}
	kind, err := parseRenderer(*primary)
	if err != nil {
		return fail(err)
	}
	mod, err := parseFeature(*modName)
	if err != nil {
		return fail(err)
	}
	lua, err := parseFeature(*luaName)
	if err != nil {
		return fail(err)
	}
	build := renderer.Build{}
	if kind == renderer.VSFilterMod {
		build = renderer.Build{Mod: mod, Lua: lua}
	} else if explicitBuild {
		return fail(fmt.Errorf("--mod-build and --lua-build require --renderer=vsfiltermod"))
	}
	profile, err := renderer.New(kind, build)
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
	doc := ass.Parse(source.Text)
	catalog, err := fontcheck.NewCatalog(fontcheck.Source{Directory: *directory})
	if err != nil {
		return fail(err)
	}
	if len(catalog.Issues) > 0 {
		return fail(fmt.Errorf("font catalog is incomplete: %s: %s (%d unreadable entries)", catalog.Issues[0].Path, catalog.Issues[0].Detail, len(catalog.Issues)))
	}
	usages := fontcheck.ExtractUsages(doc, profile)
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
