package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/spec"
)

func TestCanonicalTagStateSupportsMultiArgumentValues(t *testing.T) {
	pos := ass.Tag{Name: "pos", Args: []string{"10", "20"}}
	values, ok := CanonicalTagState(pos, spec.TagSpecs["pos"], spec.TagSpecs["pos"].Slots)
	if !ok || len(values) != 1 || values[0] != "10:10,20:20" {
		t.Fatalf("pos state = %#v, ok=%v", values, ok)
	}

	clip := ass.Tag{Name: "clip", Args: []string{"1", "2", "30", "40"}}
	values, ok = CanonicalTagState(clip, spec.TagSpecs["clip"], []string{"clip_rect"})
	if !ok || len(values) != 1 || values[0] != "clip:1:1,2:2,30:30,40:40" {
		t.Fatalf("clip state = %#v, ok=%v", values, ok)
	}
}

func TestRendererAmbiguousValuesRemainUnknown(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		slots []string
	}{
		{"blur", []string{"101"}, []string{"blur"}},
		{"a", []string{"4"}, []string{"alignment"}},
		{"clip", []string{"1.5", "0", "30", "40"}, []string{"clip_rect"}},
		{"1c", []string{"&hFFFFFF&"}, []string{"c1"}},
	} {
		if values, known := CanonicalTagState(ass.Tag{Name: tc.name, Args: tc.args}, spec.TagSpecs[tc.name], tc.slots); known {
			t.Errorf("%s %v claimed renderer-independent state: %v", tc.name, tc.args, values)
		}
	}
}
