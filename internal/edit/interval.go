package edit

// Interval is a half-open source range used for deterministic edit ordering
// and overlap detection.
type Interval struct {
	Start int
	End   int
}

func (i Interval) Overlaps(other Interval) bool {
	return i.Start < other.End && other.Start < i.End
}
