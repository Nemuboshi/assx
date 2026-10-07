package edit

import "testing"

func TestApply(t *testing.T) {
	got, err := Apply("abcdef", []TextEdit{
		{Start: 4, End: 6, Replacement: "Z"},
		{Start: 1, End: 3, Replacement: "X"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "aXdZ" {
		t.Fatalf("Apply = %q", got)
	}
}

func TestApplyRejectsOverlap(t *testing.T) {
	_, err := Apply("abcdef", []TextEdit{
		{Start: 1, End: 4},
		{Start: 3, End: 5},
	})
	if err == nil {
		t.Fatal("expected overlapping edits to fail")
	}
}
