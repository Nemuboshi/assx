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

func TestIntervalConflicts(t *testing.T) {
	if !Conflicts([]Interval{{Start: 5, End: 8}, {Start: 2, End: 6}}) {
		t.Fatal("expected conflict")
	}
	if Conflicts([]Interval{{Start: 0, End: 1}, {Start: 1, End: 2}}) {
		t.Fatal("unexpected conflict")
	}
}
