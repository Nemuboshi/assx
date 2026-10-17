package semantic

import (
	"sort"

	"assx/internal/ass/spec"
)

// slotIndex maps every known state slot to a dense ordinal. Unknown names
// fall back to map storage so a future spec addition keeps working.
type slotID int8

const (
	noLatch = -1
	// maxSlots bounds the dense tables at the exact number of modeled slots
	// (spec tables plus KeepOnStyleReset and slotExtras). Names beyond it
	// remain correct through fallback storage. The masks use one bit per
	// slot, so this must stay at most 64.
	maxSlots = 44
)

var (
	slotNames  [maxSlots]string
	slotByID   map[string]slotID
	slotExtras = []string{"clip_rect", "clip_vector", "karaoke_cursor"}
)

func init() {
	set := make(map[string]bool, maxSlots)
	for _, table := range []map[string]spec.TagSpec{spec.TagSpecs, spec.VSFilterModTagSpecs} {
		for _, tagSpec := range table {
			for _, slot := range tagSpec.Slots {
				set[slot] = true
			}
		}
	}
	for slot := range spec.KeepOnStyleReset {
		set[slot] = true
	}
	for _, slot := range slotExtras {
		set[slot] = true
	}
	names := make([]string, 0, len(set))
	for slot := range set {
		names = append(names, slot)
	}
	sort.Strings(names)
	if len(names) > maxSlots {
		names = names[:maxSlots] // Excess names stay reachable through fallback storage.
	}
	slotByID = make(map[string]slotID, len(names))
	for i, slot := range names {
		slotNames[i] = slot
		slotByID[slot] = slotID(i)
	}
}

func slotIndex(slot string) (slotID, bool) {
	id, ok := slotByID[slot]
	return id, ok
}

func slotName(id slotID) string {
	if id < 0 || int(id) >= len(slotNames) {
		return ""
	}
	return slotNames[id]
}

// slotStore replaces the per-evaluation value, owner, latch, and uncertain
// maps with dense tables plus presence bits. A slot's value state is one of
// three cases: absent (present bit clear), explicitly unknown (present,
// known clear), or known (present, known set, Value carries the text).
// Owners are dense only when the ownerPresent bit is set; latches only when
// the latched bit is set.
type slotStore struct {
	values       [maxSlots]string
	ownerIndex   [maxSlots]int32
	ownerStart   [maxSlots]int32
	latches      [maxSlots]int32
	present      uint64
	knownBits    uint64
	ownerPresent uint64
	ownerProven  uint64
	uncertain    uint64
	latched      uint64
	extra        map[string]StateValue
	extraOwners  map[string]owner
	extraUncert  map[string]bool
}

func (s *slotStore) init() {
	for i := range s.latches {
		s.latches[i] = noLatch
	}
}

func (s *slotStore) get(slot string) (StateValue, bool) {
	if id, ok := slotIndex(slot); ok {
		if s.present&(1<<id) == 0 {
			return StateValue{}, false
		}
		return StateValue{Value: s.values[id], Known: s.knownBits&(1<<id) != 0}, true
	}
	value, ok := s.extra[slot]
	return value, ok
}

func (s *slotStore) set(slot string, value StateValue) {
	if id, ok := slotIndex(slot); ok {
		s.values[id] = value.Value
		s.present |= 1 << id
		if value.Known {
			s.knownBits |= 1 << id
		} else {
			s.knownBits &^= 1 << id
		}
		return
	}
	if s.extra == nil {
		s.extra = make(map[string]StateValue)
	}
	s.extra[slot] = value
}

func (s *slotStore) delete(slot string) {
	if id, ok := slotIndex(slot); ok {
		s.values[id] = ""
		s.present &^= 1 << id
		s.knownBits &^= 1 << id
		return
	}
	delete(s.extra, slot)
}

func (s *slotStore) owner(slot string) owner {
	if id, ok := slotIndex(slot); ok {
		if s.ownerPresent&(1<<id) == 0 {
			return owner{}
		}
		return owner{index: int(s.ownerIndex[id]), start: int(s.ownerStart[id]), proven: s.ownerProven&(1<<id) != 0}
	}
	return s.extraOwners[slot]
}

func (s *slotStore) setOwner(slot string, value owner) {
	if id, ok := slotIndex(slot); ok {
		s.ownerIndex[id] = int32(value.index)
		s.ownerStart[id] = int32(value.start)
		s.ownerPresent |= 1 << id
		if value.proven {
			s.ownerProven |= 1 << id
		} else {
			s.ownerProven &^= 1 << id
		}
		return
	}
	if s.extraOwners == nil {
		s.extraOwners = make(map[string]owner)
	}
	s.extraOwners[slot] = value
}

func (s *slotStore) dropOwner(slot string) {
	if id, ok := slotIndex(slot); ok {
		s.ownerIndex[id], s.ownerStart[id] = 0, 0
		s.ownerPresent &^= 1 << id
		s.ownerProven &^= 1 << id
		return
	}
	delete(s.extraOwners, slot)
}

func (s *slotStore) hasOwner(slot string) bool {
	if id, ok := slotIndex(slot); ok {
		return s.ownerPresent&(1<<id) != 0
	}
	_, ok := s.extraOwners[slot]
	return ok
}

func (s *slotStore) latch(slot string) int {
	if id, ok := slotIndex(slot); ok && s.latched&(1<<id) != 0 {
		return int(s.latches[id])
	}
	return noLatch
}

// setLatch records a first-wins owner. Every modeled first-wins slot is in
// the dense table, so an unknown name cannot become a latch; this mirrors
// the engine, which only latches slots declared by the specification.
func (s *slotStore) setLatch(slot string, index int) {
	if id, ok := slotIndex(slot); ok {
		s.latches[id] = int32(index)
		s.latched |= 1 << id
	}
}

func (s *slotStore) uncertainLatch(slot string) bool {
	if id, ok := slotIndex(slot); ok {
		return s.uncertain&(1<<id) != 0
	}
	return s.extraUncert[slot]
}

func (s *slotStore) markUncertain(slot string) {
	if id, ok := slotIndex(slot); ok {
		s.uncertain |= 1 << id
		return
	}
	if s.extraUncert == nil {
		s.extraUncert = make(map[string]bool)
	}
	s.extraUncert[slot] = true
}

// invalidate mirrors the engine's poison rule: the value becomes explicitly
// unknown while provenance is removed.
func (s *slotStore) invalidate(slots []string) {
	for _, slot := range slots {
		s.set(slot, UnknownValue())
		s.dropOwner(slot)
	}
}

// invalidateAllFrom poisons every present value, records an unproven owner
// witness at the current position for each, and forces the two built-in
// defaults unknown. The evaluator owns the baseValid flag.
func (s *slotStore) invalidateAllFrom(position int) {
	for id := slotID(0); id < maxSlots; id++ {
		if s.present&(1<<id) == 0 {
			continue
		}
		s.values[id] = ""
		s.knownBits &^= 1 << id
		s.ownerIndex[id] = -1
		s.ownerStart[id] = int32(position)
		s.ownerPresent |= 1 << id
		s.ownerProven &^= 1 << id
	}
	for slot := range s.extra {
		s.extra[slot] = UnknownValue()
		if s.extraOwners == nil {
			s.extraOwners = make(map[string]owner)
		}
		s.extraOwners[slot] = owner{index: -1, start: position}
	}
	// These have built-in defaults and must not magically become known again
	// at a later Style reset, because the reset preserves them.
	s.set("drawing_scale", UnknownValue())
	s.set("karaoke_cursor", UnknownValue())
}

// setAssigned writes the assignment result for each target slot, dropping the
// value string when the assignment is not known.
func (s *slotStore) setAssigned(slots, values []string, known bool) {
	for i, slot := range slots {
		value := StateValue{Known: known}
		if known && i < len(values) {
			value.Value = values[i]
		}
		s.set(slot, value)
	}
}

func (s *slotStore) clear() {
	s.present, s.knownBits = 0, 0
	s.ownerPresent, s.ownerProven = 0, 0
	s.uncertain, s.latched = 0, 0
	s.extra, s.extraOwners, s.extraUncert = nil, nil, nil
	for i := range s.values {
		s.values[i] = ""
		s.ownerIndex[i], s.ownerStart[i] = 0, 0
		s.latches[i] = noLatch
	}
}

func (s *slotStore) eachPresent(fn func(slot string)) {
	for id := slotID(0); id < maxSlots; id++ {
		if s.present&(1<<id) != 0 {
			fn(slotName(id))
		}
	}
	for slot := range s.extra {
		fn(slot)
	}
}

func (s *slotStore) eachOwner(fn func(slot string, witness owner)) {
	for id := slotID(0); id < maxSlots; id++ {
		if s.ownerPresent&(1<<id) != 0 {
			fn(slotName(id), owner{index: int(s.ownerIndex[id]), start: int(s.ownerStart[id]), proven: s.ownerProven&(1<<id) != 0})
		}
	}
	for slot, witness := range s.extraOwners {
		fn(slot, witness)
	}
}
