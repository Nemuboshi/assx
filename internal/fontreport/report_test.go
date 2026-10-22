package fontreport

import (
	"bytes"
	"encoding/json/v2"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"assx/internal/ass/renderer"
	"assx/internal/fontcheck"
)

var updateFontReports = flag.Bool("update-font-reports", false, "update only font-domain report fixtures")

func TestFontReportContracts(t *testing.T) {
	u := fontcheck.Usage{Line: 1, Column: 1, SourceColumn: 1, Span: fontcheck.Span{Start: 0, End: 2}, Text: "é",
		Known: true, VisibilityKnown: true, Request: fontcheck.Request{Family: "Absent", FamilyKnown: true, BoldKnown: true, ItalicKnown: true},
		Provenance: fontcheck.Provenance{Style: "Default", Family: -1, Bold: -1, Italic: -1}}
	r := Build("sample.ass", renderer.Profile{}, fontcheck.Source{Directory: "/fonts"}, []fontcheck.Finding{{Kind: fontcheck.FamilyAbsent, Usages: []fontcheck.Usage{u}}}, []fontcheck.Usage{u})
	for _, format := range []string{"json", "plain", "pretty"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			var err error
			switch format {
			case "json":
				err = json.MarshalWrite(&out, r)
			case "plain":
				err = Plain(&out, r)
			case "pretty":
				err = Pretty(&out, r, "é", false, 80)
			}
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", format+".golden")
			if *updateFontReports {
				if err := os.MkdirAll("testdata", 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, out.Bytes(), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want, out.Bytes()) {
				t.Fatalf("%s report changed:\n%s", format, &out)
			}
		})
	}
}
