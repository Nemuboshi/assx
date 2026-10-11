package semantic

import (
	"maps"
	"reflect"
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestModScalarStateAndProvenance(t *testing.T) {
	profile := mustMod(t, renderer.FeatureEnabled)
	for _, tc := range []struct{ source, slot, value string }{
		{`{\z1.25}A`, "z_depth", "100:100"},
		{`{\frs1.29}A`, "font_orient", "12:12"},
		{`{\frs-1.29}A`, "font_orient", "-12:-12"},
		{`{\fsvp1.25}A`, "vfont_space", "10:10"},
		{`{\fshp-1.25}A`, "hfont_space", "-10:-10"},
		{`{\ortho1}A`, "ortho", "1"},
	} {
		t.Run(tc.slot+tc.value, func(t *testing.T) {
			var event TagEvent
			EvaluateResolved(ass.ParseConcreteDialogue(tc.source), profile, EvaluationOptions{
				Observer: Observer{Tag: func(e TagEvent, state StateView) {
					event = e
					if state.Value(tc.slot) != KnownValue(tc.value) || state.Source(tc.slot) != 0 {
						t.Errorf("state/source = %#v/%d", state.Value(tc.slot), state.Source(tc.slot))
					}
				}},
			})
			if event.Signature != renderer.SignatureVerified || !event.Known || !event.Applied || event.Barrier {
				t.Fatalf("verified scalar = %#v", event)
			}
		})
	}
}

func TestModStyleDefaultsAndOriginalBareReset(t *testing.T) {
	profile := mustMod(t, renderer.FeatureEnabled)
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{"fontsize": "20"}),
		"Other": {Slots: map[string]bool{"z_depth": true}, Values: map[string]StateValue{
			"z_depth": KnownValue("800:800"), "font_orient": KnownValue("30:30"),
			"vfont_space": KnownValue("8:8"), "hfont_space": KnownValue("16:16"), "ortho": KnownValue("1"),
		}},
	}
	before := make(map[string]StyleState)
	for name, style := range styles {
		before[name] = StyleState{Slots: maps.Clone(style.Slots), Values: maps.Clone(style.Values)}
	}
	var seen int
	EvaluateResolved(ass.ParseConcreteDialogue(`{\z2\rOther}A{\z\frs\fsvp\fshp\ortho2}B{\r}C`), profile, EvaluationOptions{
		Styles: styles, DialogueStyle: "Default",
		Observer: Observer{Text: func(text string, _ int, state StateView) {
			seen++
			for _, slot := range []string{"z_depth", "font_orient", "vfont_space", "hfont_space", "ortho"} {
				want := "0:0"
				if slot == "ortho" {
					want = "0"
				}
				if text == "A" {
					want = styles["Other"].Values[slot].Value
				}
				if state.Value(slot) != KnownValue(want) {
					t.Errorf("%s %s = %#v, want %s", text, slot, state.Value(slot), want)
				}
			}
			if text == "A" || text == "C" {
				if state.Source("z_depth") != -1 {
					t.Errorf("reset retained depth provenance")
				}
			} else if state.ActiveStyle() != "Other" || state.Source("z_depth") < 0 {
				t.Errorf("bare reset lost active Style or assignment provenance")
			}
		}},
	})
	if seen != 3 || !reflect.DeepEqual(styles, before) {
		t.Fatalf("missing observations or caller maps changed: %#v", styles)
	}
	for _, kind := range []renderer.Kind{renderer.Libass, renderer.XYVSFilter} {
		EvaluateResolved(ass.ParseConcreteDialogue(`{\fs30}A`), mustProfile(t, kind), EvaluationOptions{Styles: styles, DialogueStyle: "Default",
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				if state.Value("z_depth").Known {
					t.Errorf("Mod default leaked to %s", kind)
				}
			}},
		})
	}
}

func TestModDefaultsPreserveExplicitUnknownAndBuildScope(t *testing.T) {
	styles := map[string]StyleState{"Default": {Values: map[string]StateValue{"z_depth": UnknownValue()}}}
	for _, feature := range []renderer.Feature{renderer.FeatureEnabled, renderer.FeatureDisabled, renderer.FeatureUnknown} {
		profile := mustMod(t, feature)
		EvaluateResolved(ass.ParseConcreteDialogue(`{\r}A`), profile, EvaluationOptions{
			Styles: styles, DialogueStyle: "Default",
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				if state.Value("z_depth").Known {
					t.Error("explicit unknown depth became a zero default")
				}
				if state.Value("font_orient").Known != (feature == renderer.FeatureEnabled) {
					t.Errorf("orientation default leaked across build scope %v", feature)
				}
			}},
		})
	}
	if styles["Default"].Slots != nil || len(styles["Default"].Values) != 1 {
		t.Fatal("default insertion mutated caller maps")
	}
}

func TestModUnknownStyleDoesNotInventDefaults(t *testing.T) {
	profile := mustMod(t, renderer.FeatureEnabled)
	for _, source := range []string{`{\z}A`, `{\ortho2}A`, `{\rMissing\z}A`} {
		EvaluateResolved(ass.ParseConcreteDialogue(source), profile, EvaluationOptions{
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				if state.Value("z_depth").Known || state.Value("ortho").Known {
					t.Errorf("invented Style defaults: %s", source)
				}
			}},
		})
	}
}

func TestModPositionDepthLatchResetAndTransform(t *testing.T) {
	profile := mustMod(t, renderer.FeatureEnabled)
	for _, tc := range []struct {
		source, depth string
		owner         int
		firstApplied  bool
	}{
		{`{\pos(1,2,3)\pos(4,5,6)}A`, "240:240", 0, true},
		{`{\pos(1,2)\pos(4,5,6)}A`, "0:0", -1, false},
		{`{\move(1,2,3,4)\pos(4,5,6)}A`, "0:0", -1, false},
		{`{\pos(1,2,3)\r\pos(4,5,6)}A`, "0:0", -1, true},
		{`{\t(0,100,\pos(1,2,3))\pos(4,5,6)}A`, "240:240", 1, true},
	} {
		t.Run(tc.source, func(t *testing.T) {
			var depthEvents []TagEvent
			EvaluateResolved(ass.ParseConcreteDialogue(tc.source), profile, EvaluationOptions{
				Styles: map[string]StyleState{"Default": {}}, DialogueStyle: "Default",
				Observer: Observer{
					Tag: func(event TagEvent, _ StateView) {
						if event.Tag.Name == "pos" && len(event.Tag.Args) == 3 {
							depthEvents = append(depthEvents, event)
						}
					},
					Text: func(_ string, _ int, state StateView) {
						if state.Value("z_depth") != KnownValue(tc.depth) || state.Source("z_depth") != tc.owner {
							t.Errorf("depth/source = %#v/%d", state.Value("z_depth"), state.Source("z_depth"))
						}
					},
				},
			})
			if len(depthEvents) == 0 || depthEvents[0].Applied != tc.firstApplied {
				t.Fatalf("depth application = %#v", depthEvents)
			}
			last := depthEvents[len(depthEvents)-1]
			if last.Applied || !last.Ignored || last.Before != last.After {
				t.Fatalf("blocked pos3 wrote state: %#v", last)
			}
			if tc.firstApplied && (!depthEvents[0].Barrier || depthEvents[0].Uncertainty != UncertaintyRendererDependent || depthEvents[0].After[0].Known) {
				t.Fatalf("defective position acquired proof: %#v", depthEvents[0])
			}
		})
	}
	comparison := CompareSyntaxProof(ass.ParseConcreteDialogue(`{\pos(1,2,3)}A`), ass.ParseConcreteDialogue(`{\pos(1,2,3)}A`), profile, EvaluationOptions{})
	if comparison.Equivalent {
		t.Fatal("defective pos3 certified pixel equivalence")
	}
}

func TestModUnverifiedValuesAndShapesStayUnknown(t *testing.T) {
	profile := mustMod(t, renderer.FeatureEnabled)
	for _, source := range []string{`{\z(1,2)}A`, `{\z()}A`, `{\z1junk}A`, `{\z1e300}A`, `{\frs214748364.8}A`, `{\ortho2147483648}A`, `{\fshp(2,3)}A`, "{\\ortho\u00a02}A", "{\\z\u00a0}A"} {
		var last TagEvent
		EvaluateResolved(ass.ParseConcreteDialogue(source), profile, EvaluationOptions{
			Styles: map[string]StyleState{"Default": {}}, DialogueStyle: "Default",
			Observer: Observer{Tag: func(event TagEvent, _ StateView) { last = event }},
		})
		if last.Known || !last.Barrier {
			t.Errorf("unverified value/shape modeled: %s: %#v", source, last)
		}
	}
}

func TestModTransformAssignmentsAndComplexLatchBarriers(t *testing.T) {
	profile := mustMod(t, renderer.FeatureEnabled)
	for _, source := range []string{`{\t(0,100,\z2)}A`, `{\t(0,100,\frs2)}A`, `{\t(0,100,\fsvp2)}A`, `{\t(0,100,\fshp2)}A`, `{\t(0,100,\ortho1)}A`, `{\t(0,100,\pos(1,2,3)\z2)}A`} {
		EvaluateResolved(ass.ParseConcreteDialogue(source), profile, EvaluationOptions{
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				for _, slot := range []string{"z_depth", "font_orient", "vfont_space", "hfont_space", "ortho"} {
					if state.Value(slot).Known || state.Source(slot) >= 0 {
						t.Errorf("static transform value/source: %s %s", source, slot)
					}
				}
			}},
		})
	}
	for _, name := range []string{"mover", "moves3", "moves4"} {
		EvaluateResolved(ass.ParseConcreteDialogue(`{\`+name+`(1,2,3,4,5,6,7,8)\pos(1,2,3)}A`), profile, EvaluationOptions{
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				if state.Value("z_depth").Known || state.Source("z_depth") >= 0 {
					t.Errorf("uncertain complex latch published depth: %s", name)
				}
			}},
		})
	}
}

func TestMoveRejectedShapeDoesNotClaimPositionLatch(t *testing.T) {
	for _, profile := range []renderer.Profile{mustProfile(t, renderer.Libass), mustProfile(t, renderer.XYVSFilter), mustMod(t, renderer.FeatureEnabled)} {
		var events []TagEvent
		EvaluateResolved(ass.ParseConcreteDialogue(`{\move(1,2,3,4,5,6,7,8)\pos(9,10)}A`), profile, EvaluationOptions{
			Observer: Observer{Tag: func(event TagEvent, state StateView) {
				events = append(events, event)
				if event.Index == 1 && (!state.Value("position").Known || state.Source("position") != 1) {
					t.Errorf("rejected move claimed %s position latch", profile.Kind())
				}
			}},
		})
		if len(events) != 2 || !events[0].Ignored || events[0].Signature != renderer.SignatureRejected || !events[1].Applied {
			t.Errorf("move rejection/latch: %#v", events)
		}
	}
}

func TestModUnverifiedPositionCannotPublishDepth(t *testing.T) {
	profile := mustMod(t, renderer.FeatureEnabled)
	for _, source := range []string{`{\posSuffix(1,2,3)}A`, "{\\pos(1,2,\u00a03)}A"} {
		EvaluateResolved(ass.ParseConcreteDialogue(source), profile, EvaluationOptions{
			Observer: Observer{Text: func(_ string, _ int, state StateView) {
				if state.Value("z_depth").Known || state.Source("z_depth") >= 0 {
					t.Errorf("unverified position published depth: %s", source)
				}
			}},
		})
	}
}

func TestModLuaShadowsCoreAndUnknownNames(t *testing.T) {
	for _, mod := range []renderer.Feature{renderer.FeatureEnabled, renderer.FeatureDisabled, renderer.FeatureUnknown} {
		for _, lua := range []renderer.Feature{renderer.FeatureEnabled, renderer.FeatureUnknown} {
			profile, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: mod, Lua: lua})
			if err != nil {
				t.Fatal(err)
			}
			EvaluateResolved(ass.ParseConcreteDialogue(`{\fs20\pos(1,2)\mystery\z3}A`), profile, EvaluationOptions{
				Styles: map[string]StyleState{"Default": {}}, DialogueStyle: "Default",
				Observer: Observer{Tag: func(event TagEvent, state StateView) {
					if !event.Barrier || event.Uncertainty != UncertaintyRendererDependent || event.Applied {
						t.Errorf("Lua shadow lacks barrier: %#v", event)
					}
					for _, slot := range []string{"fontsize", "position", "z_depth"} {
						if state.Value(slot).Known || state.Source(slot) >= 0 {
							t.Errorf("Lua shadow retained state: %s", slot)
						}
					}
				}},
			})
		}
	}
}

func TestModFadRemainsReachableAndLatched(t *testing.T) {
	for _, profile := range []renderer.Profile{mustProfile(t, renderer.Libass), mustProfile(t, renderer.XYVSFilter), mustMod(t, renderer.FeatureEnabled), mustMod(t, renderer.FeatureDisabled)} {
		var events []TagEvent
		EvaluateResolved(ass.ParseConcreteDialogue(`{\fad(10,20)\r\fad(30,40)}A`), profile, EvaluationOptions{Observer: Observer{Tag: func(event TagEvent, _ StateView) { events = append(events, event) }}})
		if len(events) != 3 || events[0].Match != renderer.Matched || !events[0].Applied || events[2].Applied {
			t.Errorf("fad dispatch/latch changed for %s: %#v", profile.Kind(), events)
		}
	}
}
