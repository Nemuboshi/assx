package sourcepos

import (
	"testing"

	"assx/internal/edit"
)

func TestPositionsRoundTrip(t *testing.T) {
	text := "A\t中😀\r\n日\rZ\n"
	index, err := New(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		offset int
		want   Position
	}{
		{0, Position{0, 0}},
		{1, Position{0, 1}},
		{2, Position{0, 2}},
		{5, Position{0, 3}},
		{9, Position{0, 5}},
		{11, Position{1, 0}},
		{14, Position{1, 1}},
		{15, Position{2, 0}},
		{16, Position{2, 1}},
		{17, Position{3, 0}},
	} {
		got, err := index.Position(tc.offset)
		if err != nil || got != tc.want {
			t.Fatalf("Position(%d) = %+v, %v; want %+v", tc.offset, got, err, tc.want)
		}
		offset, err := index.Offset(got)
		if err != nil || offset != tc.offset {
			t.Fatalf("Offset(%+v) = %d, %v; want %d", got, offset, err, tc.offset)
		}
	}
}

func TestPositionsRejectInvalidBoundaries(t *testing.T) {
	index, err := New("中😀\r\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{-1, 1, 4, 8, 10} {
		if _, err := index.Position(offset); err == nil {
			t.Errorf("Position(%d) accepted invalid boundary", offset)
		}
	}
	for _, pos := range []Position{{-1, 0}, {0, -1}, {0, 2}, {0, 4}, {0, 100}, {2, 0}} {
		if _, err := index.Offset(pos); err == nil {
			t.Errorf("Offset(%+v) accepted invalid position", pos)
		}
	}
	if _, err := New("\xff"); err == nil {
		t.Fatal("invalid UTF-8 accepted")
	}
}

func TestEmptyAndEditRanges(t *testing.T) {
	index, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := index.Position(0); err != nil || got != (Position{}) {
		t.Fatalf("empty Position = %+v, %v", got, err)
	}
	if got, err := index.EditRange(edit.TextEdit{Start: 0, End: 0, Replacement: "x"}); err != nil || got != (Range{}) {
		t.Fatalf("insertion range = %+v, %v", got, err)
	}
	index, err = New("a😀b")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := index.EditRange(edit.TextEdit{Start: 1, End: 5}); err != nil || got != (Range{Start: Position{0, 1}, End: Position{0, 3}}) {
		t.Fatalf("replacement range = %+v, %v", got, err)
	}
	for _, span := range []edit.TextEdit{{Start: 2, End: 5}, {Start: 1, End: 4}, {Start: 5, End: 1}, {Start: -1, End: 0}, {Start: 0, End: 100}} {
		if _, err := index.EditRange(span); err == nil {
			t.Errorf("edit %+v accepted invalid range", span)
		}
	}
}

func TestDecodedBOMAndJapaneseText(t *testing.T) {
	index, err := New("\ufeff日\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		offset int
		want   Position
	}{
		{0, Position{0, 0}},
		{3, Position{0, 1}},
		{6, Position{0, 2}},
		{7, Position{1, 0}},
	} {
		if got, err := index.Position(tc.offset); err != nil || got != tc.want {
			t.Fatalf("Position(%d) = %+v, %v; want %+v", tc.offset, got, err, tc.want)
		}
	}
}

func TestUninitializedIndexRejectsPositions(t *testing.T) {
	var index Index
	if _, err := index.Position(0); err == nil {
		t.Fatal("zero index accepted a byte offset")
	}
	if _, err := index.Offset(Position{}); err == nil {
		t.Fatal("zero index accepted a position")
	}
}
