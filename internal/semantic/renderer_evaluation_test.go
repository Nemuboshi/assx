package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestRendererScopedPositionFirstWins(t *testing.T) {
	tests := []struct {
		name             string
		profile          renderer.Profile
		firstIgnored     bool
		firstUncertainty SemanticUncertainty
		secondApplied    bool
		known            bool
		wantSource       int
	}{
		{
			name: "libass", profile: mustProfile(t, renderer.Libass),
			firstIgnored: true, firstUncertainty: UncertaintyMalformed,
			secondApplied: true, known: true, wantSource: 1,
		},
		{
			name: "xy-VSFilter", profile: mustProfile(t, renderer.XYVSFilter),
			firstIgnored: true, firstUncertainty: UncertaintyMalformed,
			secondApplied: true, known: true, wantSource: 1,
		},
		{
			name:             "VSFilterMod enabled",
			profile:          mustMod(t, renderer.FeatureEnabled),
			firstUncertainty: UncertaintyNone, secondApplied: false,
			known: true, wantSource: 0,
		},
		{
			name:         "VSFilterMod disabled",
			profile:      mustMod(t, renderer.FeatureDisabled),
			firstIgnored: true, firstUncertainty: UncertaintyMalformed,
			secondApplied: true, known: true, wantSource: 1,
		},
		{
			name:             "VSFilterMod unknown build",
			profile:          mustMod(t, renderer.FeatureUnknown),
			firstUncertainty: UncertaintyUnresolved, secondApplied: false,
			known: false, wantSource: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []TagEvent
			seen := false
			Evaluate(ass.ParseDialogueText(`{\pos(1,2,3)\pos(4,5)}A`), EvaluationOptions{
				Profile: &tt.profile,
				Observer: Observer{
					Tag: func(event TagEvent, _ StateView) {
						events = append(events, event)
					},
					Text: func(text string, _ int, state StateView) {
						if text != "A" {
							return
						}
						seen = true
						v := state.Value("position")
						if v.Known != tt.known {
							t.Errorf("position known=%v, want %v: %#v", v.Known, tt.known, v)
						}
						if src := state.Source("position"); src != tt.wantSource {
							t.Errorf("source=%d, want %d", src, tt.wantSource)
						}
					},
				},
			})
			if !seen || len(events) != 2 {
				t.Fatalf("unexpected text/tag events: text=%v tags=%#v", seen, events)
			}
			if events[0].Ignored != tt.firstIgnored || events[0].Uncertainty != tt.firstUncertainty {
				t.Errorf("first pos = %#v", events[0])
			}
			if events[1].Applied != tt.secondApplied {
				t.Errorf("second pos applied=%v, want %v", events[1].Applied, tt.secondApplied)
			}
		})
	}
}

func mustProfile(t *testing.T, kind renderer.Kind) renderer.Profile {
	t.Helper()
	profile, err := renderer.Standard(kind)
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func mustMod(t *testing.T, state renderer.Feature) renderer.Profile {
	t.Helper()
	profile, err := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: state})
	if err != nil {
		t.Fatal(err)
	}
	return profile
}

func TestRendererScopedPrefixShadowingAndUnmodeledExtension(t *testing.T) {
	for _, kind := range []renderer.Kind{renderer.Libass, renderer.XYVSFilter} {
		profile := mustProfile(t, kind)
		var events []TagEvent
		Evaluate(ass.ParseDialogueText(`{\fsvp6\frs10\blend(add)}A`), EvaluationOptions{
			Profile: &profile,
			Observer: Observer{Tag: func(event TagEvent, _ StateView) {
				events = append(events, event)
			}},
		})
		want := []struct{ name, shadowed string }{
			{"fs", "fsvp"}, {"fr", "frs"}, {"b", "blend"},
		}
		if len(events) != len(want) {
			t.Fatalf("%s events: %#v", kind, events)
		}
		for i, e := range events {
			if e.Tag.Name != want[i].name || e.Shadowed != want[i].shadowed {
				t.Errorf("%s event %d name=%q shadowed=%q", kind, i, e.Tag.Name, e.Shadowed)
			}
		}
	}

	profile := mustMod(t, renderer.FeatureEnabled)
	var events []TagEvent
	Evaluate(ass.ParseDialogueText(`{\fsvp6\frs10\blend(add)}A`), EvaluationOptions{
		Profile:  &profile,
		Observer: Observer{Tag: func(e TagEvent, _ StateView) { events = append(events, e) }},
	})
	if len(events) != 3 {
		t.Fatalf("Mod tags = %#v", events)
	}
	for i, want := range []string{"fsvp", "frs", "blend"} {
		if events[i].Tag.Name != want || events[i].Uncertainty != UncertaintyUnsupported || !events[i].Barrier {
			t.Errorf("Mod extension %d = %#v", i, events[i])
		}
	}
}

func TestRendererScopedTransformAndStateProvenance(t *testing.T) {
	profile := mustProfile(t, renderer.Libass)
	var events []TagEvent
	Evaluate(ass.ParseDialogueText(`{\clip(0,0,10,10)\clip(1,2,3)\t(0,500,\fs30)}A`), EvaluationOptions{
		Profile: &profile,
		Observer: Observer{
			Tag: func(e TagEvent, state StateView) {
				events = append(events, e)
				if e.Index >= 1 {
					for _, slot := range []string{"clip_rect", "clip_vector"} {
						if state.Value(slot).Known || state.Source(slot) != -1 {
							t.Errorf("stale clip value/source after event %d: %s", e.Index, slot)
						}
					}
				}
			},
		},
	})
	if len(events) != 4 {
		t.Fatalf("transform walk missed resolved child: %#v", events)
	}
	if events[1].Uncertainty != UncertaintyUnresolved || !events[1].Barrier {
		t.Errorf("invalid clip not conservatively handled: %#v", events[1])
	}
	if events[3].Tag.Name != "fs" || !events[3].Tag.InTransition {
		t.Errorf("child interpretation lost: %#v", events[3])
	}
}

func TestRendererScopedStyleFontObserver(t *testing.T) {
	profile := mustProfile(t, renderer.XYVSFilter)
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{"fontname": "Arial", "fontsize": "20"}),
	}
	var family string
	Evaluate(ass.ParseDialogueText(`{\fnOther\r}Hello`), EvaluationOptions{
		Profile: &profile, Styles: styles, DialogueStyle: "Default",
		SkipNoEffectProofs: true,
		Observer: Observer{Text: func(text string, _ int, state StateView) {
			if text == "Hello" && state.Value("fontname").Known {
				family = state.Value("fontname").Value
			}
		}},
	})
	if family != "Arial" {
		t.Fatalf("reset didn't restore style font: %q", family)
	}
}

func TestRendererScopedEstablishedOwnerSurvivesUncertainLaterPosition(t *testing.T) {
	profile := mustMod(t, renderer.FeatureUnknown)
	var after StateValue
	var owner int
	EvaluateResolved(ass.ParseConcreteDialogue(`{\pos(4,5)\pos(1,2,3)}A`), profile, EvaluationOptions{
		Observer: Observer{Text: func(text string, _ int, state StateView) {
			if text == "A" {
				after = state.Value("position")
				owner = state.Source("position")
			}
		}},
	})
	if !after.Known || owner != 0 {
		t.Fatalf("known first-wins owner was destroyed by an uncertain later command: value=%#v owner=%d", after, owner)
	}
}

func TestRendererScopedRepeatedSlashesRemainUnresolved(t *testing.T) {
	profile := mustProfile(t, renderer.Libass)
	var event TagEvent
	EvaluateResolved(ass.ParseConcreteDialogue(`{\\fs20}A`), profile, EvaluationOptions{
		Observer: Observer{Tag: func(e TagEvent, _ StateView) { event = e }},
	})
	if event.Uncertainty != UncertaintyUnresolved || !event.Barrier {
		t.Fatalf("ambiguous slash-run was treated as harmless: %#v", event)
	}
}

func TestRendererScopedUnclosedClipInvalidatesStateAndProvenance(t *testing.T) {
	for _, profile := range []renderer.Profile{
		mustProfile(t, renderer.Libass),
		mustProfile(t, renderer.XYVSFilter),
		mustMod(t, renderer.FeatureEnabled),
	} {
		for _, command := range []string{"clip", "iclip"} {
			t.Run(profile.Kind().String()+"/"+command, func(t *testing.T) {
				source := "{\\fs20\\clip(0,0,10,10)\\" + command + "(1,2,3}A"
				var malformed bool
				var checkedText bool
				assertInvalid := func(where string, state StateView) {
					t.Helper()
					for _, slot := range []string{"clip_rect", "clip_vector"} {
						if value := state.Value(slot); value.Known {
							t.Errorf("%s: malformed %s retained known %s: %#v", where, command, slot, value)
						}
						if source := state.Source(slot); source != -1 {
							t.Errorf("%s: malformed %s retained source for %s: %d", where, command, slot, source)
						}
					}
					if fontSize := state.Value("fontsize"); !fontSize.Known {
						t.Errorf("%s: malformed clip invalidated unrelated font size", where)
					}
				}
				EvaluateResolved(ass.ParseConcreteDialogue(source), profile, EvaluationOptions{
					Observer: Observer{
						Tag: func(event TagEvent, state StateView) {
							if event.Tag.Name != command || event.Index != 2 {
								return
							}
							malformed = true
							if event.Ignored || !event.Barrier || event.Uncertainty != UncertaintyMalformed {
								t.Errorf("unclosed %s event incorrectly ignored: %#v", command, event)
							}
							assertInvalid("tag", state)
						},
						Text: func(text string, _ int, state StateView) {
							if text == "A" {
								checkedText = true
								assertInvalid("text", state)
							}
						},
					},
				})
				if !malformed || !checkedText {
					t.Fatalf("missing malformed tag/text observations: %q", source)
				}
			})
		}
	}
}

func TestRendererScopedUnclosedTransformInvalidatesAllState(t *testing.T) {
	for _, profile := range []renderer.Profile{
		mustProfile(t, renderer.Libass),
		mustProfile(t, renderer.XYVSFilter),
		mustMod(t, renderer.FeatureEnabled),
	} {
		t.Run(profile.Kind().String(), func(t *testing.T) {
			var sawMalformed, sawText bool
			EvaluateResolved(ass.ParseConcreteDialogue("{\\fs20\\t(0,100,\\fs30}A"), profile, EvaluationOptions{
				Observer: Observer{
					Tag: func(event TagEvent, state StateView) {
						if event.Tag.Name == "t" {
							sawMalformed = true
							if event.Ignored || !event.Barrier || event.Uncertainty != UncertaintyMalformed {
								t.Errorf("unclosed transform lacks malformed barrier: %#v", event)
							}
							if state.Value("fontsize").Known || state.Source("fontsize") != -1 {
								t.Errorf("transform retained font size or provenance")
							}
						}
					},
					Text: func(text string, _ int, state StateView) {
						if text == "A" {
							sawText = true
							if state.Value("fontsize").Known || state.Source("fontsize") != -1 {
								t.Errorf("malformed transform recovered unproved state at text")
							}
						}
					},
				},
			})
			if !sawMalformed || !sawText {
				t.Fatal("missing transform or text callback")
			}
		})
	}
}

func TestRendererScopedUnclosedSyntaxRevokesEarlierNoEffectProofs(t *testing.T) {
	for _, profile := range []renderer.Profile{
		mustProfile(t, renderer.Libass),
		mustProfile(t, renderer.XYVSFilter),
		mustMod(t, renderer.FeatureEnabled),
	} {
		t.Run(profile.Kind().String(), func(t *testing.T) {
			evaluation := EvaluateResolved(ass.ParseConcreteDialogue("{\\fs20\\fs20\\clip(1,2,3}A"), profile, EvaluationOptions{})
			if len(evaluation.NoEffects) != 1 || !evaluation.NoEffects[0].ProofRevoked {
				t.Fatalf("unclosed clip did not retroactively revoke no-effect proof: %#v", evaluation.NoEffects)
			}
		})
	}
}

func TestRendererScopedClosedUnknownCommandsRemainIgnored(t *testing.T) {
	profile := mustProfile(t, renderer.Libass)
	for _, source := range []string{
		"{\\fs20\\fs20\\mystery}A",
		"{\\fs20\\fs20\\mystery(1,2)}A",
	} {
		t.Run(source, func(t *testing.T) {
			var unknown *TagEvent
			evaluation := EvaluateResolved(ass.ParseConcreteDialogue(source), profile, EvaluationOptions{
				Observer: Observer{Tag: func(event TagEvent, _ StateView) {
					if event.Tag.Name == "mystery" {
						copy := event
						unknown = &copy
					}
				}},
			})
			if unknown == nil || !unknown.Ignored || unknown.Barrier {
				t.Fatalf("well-formed unknown command became a syntax barrier: %#v", unknown)
			}
			if len(evaluation.NoEffects) != 1 || evaluation.NoEffects[0].ProofRevoked {
				t.Fatalf("well-formed unknown command revoked supported no-effect proof: %#v", evaluation.NoEffects)
			}
		})
	}
}
