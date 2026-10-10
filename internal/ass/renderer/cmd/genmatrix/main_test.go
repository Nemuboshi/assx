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

func TestRequirementMaskRejectsUnsupportedOrAmbiguousGates(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		mask  uint8
		valid bool
	}{
		{"", 0, true},
		{"_VSMOD", 1, true},
		{"_VSMOD _LUA", 3, true},
		{"_LUA", 0, false},
		{"_LUA _VSMOD", 0, false},
		{"_VSMOD _VSMOD", 0, false},
		{"_VSMOD garbage", 0, false},
	} {
		got, err := requirementMask(tc.raw)
		if (err == nil) != tc.valid || (tc.valid && got != tc.mask) {
			t.Errorf("requires=%q got mask=%d err=%v, want mask=%d valid=%v", tc.raw, got, err, tc.mask, tc.valid)
		}
	}
}
