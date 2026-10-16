package edit

import "testing"

func TestIntervalOverlap(t *testing.T) {
	if !(Interval{Start: 1, End: 4}).Overlaps(Interval{Start: 3, End: 5}) {
		t.Fatal("expected overlapping intervals")
	}
	if (Interval{Start: 1, End: 3}).Overlaps(Interval{Start: 3, End: 5}) {
		t.Fatal("touching intervals should not overlap")
	}
}
