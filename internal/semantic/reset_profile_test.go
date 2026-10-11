package semantic

import (
	"testing"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

func TestResolvedResetRetainsOriginalStyleAndFirstArgument(t *testing.T) {
	styles := map[string]StyleState{
		"Default": CanonicalStyleState(map[string]string{"fontsize": "20"}),
		"Other":   CanonicalStyleState(map[string]string{"fontsize": "30"}),
	}
	libass, _ := renderer.Standard(renderer.Libass)
	xy, _ := renderer.Standard(renderer.XYVSFilter)
	mod, _ := renderer.New(renderer.VSFilterMod, renderer.Build{Mod: renderer.FeatureEnabled, Lua: renderer.FeatureDisabled})
	for _, profile := range []renderer.Profile{libass, xy, mod} {
		t.Run(profile.Kind().String(), func(t *testing.T) {
			var names []string
			EvaluateResolved(ass.ParseConcreteDialogue(`{\r(Other,unused)}A{\r}B`), profile, EvaluationOptions{
				Styles: styles, DialogueStyle: "Default",
				Observer: Observer{Text: func(_ string, _ int, state StateView) {
					names = append(names, state.ActiveStyle())
					if state.OriginalStyle() != "Default" || state.Original().Values["fontsize"] != styles["Default"].Values["fontsize"] {
						t.Fatal("named reset replaced original Style")
					}
				}},
			})
			if len(names) != 2 || names[0] != "Other" || names[1] != "Default" {
				t.Fatalf("reset Styles = %v", names)
			}
		})
	}
}
