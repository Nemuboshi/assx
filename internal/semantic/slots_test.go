package semantic

import (
	"slices"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
	"assx/internal/ass/spec"
)

func TestSlotIndexMapsEveryKnownSlot(t *testing.T) {
	seen := make(map[slotID]string)
	names := make(map[string]bool)
	collect := func(slot string) { names[slot] = true }
	for _, table := range []map[string]spec.TagSpec{spec.TagSpecs, spec.VSFilterModTagSpecs} {
		for _, tagSpec := range table {
			for _, slot := range tagSpec.Slots {
				collect(slot)
			}
		}
	}
	for slot := range spec.KeepOnStyleReset {
		collect(slot)
	}
	for _, slots := range stylePropertySlots {
		for _, slot := range slots {
			collect(slot)
		}
	}
	for _, slot := range []string{
		"karaoke_cursor", "clip_rect", "clip_vector", "position", "z_depth",
		"fontsize", "fade", "alignment", "origin", "drawing_scale",
	} {
		collect(slot)
	}
	for slot := range names {
		id, ok := slotIndex(slot)
		if !ok {
			t.Errorf("known slot %q has no index", slot)
			continue
		}
		if previous, dup := seen[id]; dup {
			t.Fatalf("slots %q and %q share index %d", previous, slot, id)
		}
		seen[id] = slot
		if got := slotName(id); got != slot {
			t.Errorf("slotIndex(%q)=%d round-trips to %q", slot, id, got)
		}
	}
	if _, ok := slotIndex("definitely-not-a-slot"); ok {
		t.Error("unknown slot resolved to an index")
	}
}

func TestSlotStoreUnknownNameUsesFallback(t *testing.T) {
	var store slotStore
	store.init()
	if _, ok := store.get("mystery_slot"); ok {
		t.Error("fresh store reports a value for an absent slot")
	}
	store.set("mystery_slot", KnownValue("7"))
	value, ok := store.get("mystery_slot")
	if !ok || value != KnownValue("7") {
		t.Fatalf("fallback get = %#v, %v", value, ok)
	}
	store.invalidate([]string{"mystery_slot"})
	if got, ok := store.get("mystery_slot"); !ok || got.Known {
		t.Fatalf("invalidate left fallback storage reachable: %#v %v", got, ok)
	}
	store.set("mystery_slot", KnownValue("8"))
	store.clear()
	if _, ok := store.get("mystery_slot"); ok {
		t.Error("clear removed only indexed storage")
	}
	if store.uncertainLatch("mystery_slot") {
		t.Error("fallback uncertainty lost")
	}
}

func TestSlotStorePresenceDistinguishesUnknownFromAbsent(t *testing.T) {
	var store slotStore
	store.init()
	store.set("bold", UnknownValue())
	if _, ok := store.get("bold"); !ok {
		t.Fatal("explicit unknown value lost its presence")
	}
	store.delete("bold")
	if _, ok := store.get("bold"); ok {
		t.Fatal("delete left presence set")
	}
	store.set("bold", KnownValue("1"))
	store.delete("bold")
	if _, ok := store.get("bold"); ok {
		t.Fatal("delete of a known value left presence set")
	}
}

func TestSlotStoreOwnerPresenceTracksProofStatus(t *testing.T) {
	var store slotStore
	store.init()
	if fresh := store.owner("position"); fresh.proven || fresh.index != 0 {
		t.Fatalf("fresh owner = %#v, want zero owner", fresh)
	}
	store.setOwner("position", owner{index: 3, start: 2, proven: true})
	if got := store.owner("position"); got != (owner{index: 3, start: 2, proven: true}) {
		t.Fatalf("owner round trip = %#v", got)
	}
	if !store.hasOwner("position") || store.hasOwner("origin") {
		t.Fatal("hasOwner disagrees with setOwner")
	}
	store.dropOwner("position")
	if store.hasOwner("position") {
		t.Fatal("dropOwner left presence")
	}
	store.setOwner("fade", owner{index: -1, start: 1})
	if !store.hasOwner("fade") {
		t.Fatal("unproven owner must stay visible to provenance walks")
	}
}

func TestSlotStoreLatchAndUncertainTables(t *testing.T) {
	var store slotStore
	store.init()
	if store.latch("position") != noLatch {
		t.Fatal("fresh latch must read noLatch")
	}
	store.setLatch("position", 5)
	if store.latch("position") != 5 {
		t.Fatal("latch round trip failed")
	}
	store.markUncertain("position")
	if !store.uncertainLatch("position") {
		t.Fatal("uncertain latch flag lost")
	}
	store.clear()
	if store.latch("position") != noLatch || store.uncertainLatch("position") {
		t.Fatal("clear reset only value storage")
	}
}

func TestSlotStoreInvalidateAllKeepsDefaultsUnknown(t *testing.T) {
	var store slotStore
	store.init()
	store.set("drawing_scale", KnownValue("1"))
	store.setOwner("drawing_scale", owner{index: 1, start: 0, proven: true})
	store.set("bold", KnownValue("1"))
	store.setOwner("bold", owner{index: 2, start: 0, proven: true})
	store.invalidateAllFrom(4)
	for _, slot := range []string{"drawing_scale", "karaoke_cursor", "bold"} {
		if value, ok := store.get(slot); !ok || value.Known {
			t.Fatalf("%s after invalidateAll = %#v %v", slot, value, ok)
		}
	}
	if !store.hasOwner("bold") || !store.hasOwner("drawing_scale") {
		t.Fatal("invalidateAll must keep each poisoned owner as a witness")
	}
	if got := store.owner("bold"); got.index != -1 || got.start != 4 || got.proven {
		t.Fatalf("owner witness = %#v", got)
	}
}

func TestSlotStoreEachPresentAndOwnersVisitIndexedAndFallback(t *testing.T) {
	var store slotStore
	store.init()
	store.set("bold", KnownValue("1"))
	store.set("clip_vector", UnknownValue())
	store.set("mystery_slot", KnownValue("9"))
	var visited []string
	store.eachPresent(func(slot string) { visited = append(visited, slot) })
	slices.Sort(visited)
	if want := []string{"bold", "clip_vector", "mystery_slot"}; !slices.Equal(visited, want) {
		t.Fatalf("eachPresent = %v, want %v", visited, want)
	}
	store.setOwner("bold", owner{index: 0, start: 0, proven: true})
	var owned []string
	store.eachOwner(func(slot string, _ owner) { owned = append(owned, slot) })
	slices.Sort(owned)
	if want := []string{"bold"}; !slices.Equal(owned, want) {
		t.Fatalf("eachOwner = %v, want %v", owned, want)
	}
}

func TestStateViewIndexedFallbackChain(t *testing.T) {
	styles := map[string]StyleState{
		"Default": {Slots: map[string]bool{"bold": true}, Values: map[string]StateValue{"bold": KnownValue("1")}},
	}
	var store slotStore
	store.init()
	view := StateView{store: &store, styles: styles, activeStyle: "Default", originalStyle: "Default", baseValid: true}
	if got := view.Value("bold"); got != KnownValue("1") {
		t.Fatalf("style base read = %#v", got)
	}
	if got := view.Value("drawing_scale"); got != KnownValue("0") {
		t.Fatalf("default with valid base = %#v", got)
	}
	store.set("bold", KnownValue("0"))
	if got := view.Value("bold"); got != KnownValue("0") {
		t.Fatalf("override read = %#v", got)
	}
	store.invalidateAllFrom(1)
	view.baseValid = false
	if got := view.Value("bold"); got.Known {
		t.Fatalf("poisoned override must stay unknown: %#v", got)
	}
	if got := view.Value("spacing"); got.Known {
		t.Fatalf("invalid base must hide unassigned style slots: %#v", got)
	}
	view.baseValid = true
	store.delete("bold")
	if got := view.Value("bold"); got != KnownValue("1") {
		t.Fatalf("deleted override must expose the base again: %#v", got)
	}
}

func TestStateViewSourceAbsentOwners(t *testing.T) {
	var store slotStore
	store.init()
	view := StateView{store: &store, styles: map[string]StyleState{"Default": {}}, activeStyle: "Default"}
	if got := view.Source("position"); got != -1 {
		t.Fatalf("absent owner = %d", got)
	}
	store.setOwner("position", owner{index: -1, start: 0})
	if got := view.Source("position"); got != -1 {
		t.Fatalf("unproven owner = %d", got)
	}
	store.setOwner("position", owner{index: 2, start: 0, proven: true})
	if got := view.Source("position"); got != 2 {
		t.Fatalf("proven owner = %d", got)
	}
	if got := view.Source("mystery_slot"); got != -1 {
		t.Fatalf("unknown slot owner = %d", got)
	}
}

func TestStateViewSnapshotRetainsCopiedOwnership(t *testing.T) {
	profile, err := renderer.Standard(renderer.Libass)
	if err != nil {
		t.Fatal(err)
	}
	var snapshots []StateSnapshot
	EvaluateResolved(ass.ParseConcreteDialogue(`{\b1}x{\i1}y`), profile, EvaluationOptions{
		DialogueStyle: "Default",
		Observer:      Observer{Text: func(_ string, _ int, v StateView) { snapshots = append(snapshots, v.Snapshot()) }},
	})
	if len(snapshots) != 2 {
		t.Fatalf("snapshot count = %d", len(snapshots))
	}
	if got := snapshots[0].Values["italic"]; got.Known {
		t.Fatalf("first snapshot saw a later assignment: %#v", got)
	}
	if got := snapshots[1].Values["italic"]; !got.Known || got.Value != "1" {
		t.Fatalf("second snapshot lost the later assignment: %#v", got)
	}
	if got := snapshots[0].Sources["bold"]; got != 0 {
		t.Fatalf("first snapshot owner ordinal = %d", got)
	}
}
