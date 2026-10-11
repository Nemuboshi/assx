package edit

// Interval is a half-open source range used for deterministic edit ordering
// and conflict detection.
type Interval struct {
	Start int
	End   int
}

func (i Interval) Valid(limit int) bool {
	return i.Start >= 0 && i.End >= i.Start && i.End <= limit
}

func (i Interval) Overlaps(other Interval) bool {
	return i.Start < other.End && other.Start < i.End
}

// Conflicts reports whether any intervals overlap after sorting by source position.
func Conflicts(intervals []Interval) bool {
	ordered := append([]Interval(nil), intervals...)
	for i := 1; i < len(ordered); i++ {
		for j := i; j > 0 && (ordered[j].Start < ordered[j-1].Start || (ordered[j].Start == ordered[j-1].Start && ordered[j].End < ordered[j-1].End)); j-- {
			ordered[j], ordered[j-1] = ordered[j-1], ordered[j]
		}
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1].End > ordered[i].Start {
			return true
		}
	}
	return false
}
