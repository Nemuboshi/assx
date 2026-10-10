package main

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestCanonicalNewlinesAreIndependentOfCheckoutStyle(t *testing.T) {
	lf := []byte("<matrix>\n  <tag name=\"test\"/>\n</matrix>\n")
	want := sha256.Sum256(lf)
	for name, source := range map[string][]byte{
		"LF":   lf,
		"CRLF": bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n")),
		"CR":   bytes.ReplaceAll(lf, []byte("\n"), []byte("\r")),
	} {
		t.Run(name, func(t *testing.T) {
			got := canonicalNewlines(source)
			if !bytes.Equal(got, lf) {
				t.Fatalf("canonical XML differs: %q", got)
			}
			if sha256.Sum256(got) != want {
				t.Fatal("evidence digest depends on physical newline style")
			}
		})
	}
}
